package kafka

import (
	"encoding/json"
	"fmt"

	"github.com/Shopify/sarama"
	sc "github.com/bsm/sarama-cluster"
	"github.com/micro/go-micro/broker"
	"github.com/micro/go-micro/cmd"
)

type kBroker struct {
	addrs []string

	c  sarama.Client
	p  sarama.SyncProducer
	sc *sc.Client

	opts broker.Options
}

type subscriber struct {
	s    *sc.Consumer
	t    string
	opts broker.SubscribeOptions
}

type publication struct {
	t string
	m *broker.Message
}

func init() {
	cmd.DefaultBrokers["kafka"] = NewBroker
}

func (p *publication) Topic() string {
	return p.t
}

func (p *publication) Message() *broker.Message {
	return p.m
}

func (p *publication) Ack() error {
	return nil
}

func (s *subscriber) Options() broker.SubscribeOptions {
	return s.opts
}

func (s *subscriber) Topic() string {
	return s.t
}

func (s *subscriber) Unsubscribe() error {
	return s.s.Close()
}

func (k *kBroker) Address() string {
	if len(k.addrs) > 0 {
		return k.addrs[0]
	}
	return "127.0.0.1:9092"
}

func (k *kBroker) Connect() error {
	if k.c != nil {
		return nil
	}

	c, err := sarama.NewClient(k.addrs, sarama.NewConfig())
	if err != nil {
		return err
	}

	k.c = c

	p, err := sarama.NewSyncProducerFromClient(c)
	if err != nil {
		return err
	}

	k.p = p

	cs, err := sc.NewClient(k.addrs, sc.NewConfig())
	if err != nil {
		return err
	}

	k.sc = cs
	// TODO: TLS
	/*
		opts.Secure = k.opts.Secure
		opts.TLSConfig = k.opts.TLSConfig

		// secure might not be set
		if k.opts.TLSConfig != nil {
			opts.Secure = true
		}
	*/
	return nil
}

func (k *kBroker) Disconnect() error {
	k.sc.Close()
	k.p.Close()
	return k.c.Close()
}

func (k *kBroker) Init(opts ...broker.Option) error {
	for _, o := range opts {
		o(&k.opts)
	}
	return nil
}

func (k *kBroker) Options() broker.Options {
	return k.opts
}

func (k *kBroker) Publish(topic string, msg *broker.Message, opts ...broker.PublishOption) error {
	b, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	_, _, err = k.p.SendMessage(&sarama.ProducerMessage{
		Topic: topic,
		Value: sarama.ByteEncoder(b),
	})
	return err
}

func (k *kBroker) Subscribe(topic string, handler broker.Handler, opts ...broker.SubscribeOption) (broker.Subscriber, error) {
	opt := broker.SubscribeOptions{
		AutoAck: true,
	}

	for _, o := range opts {
		o(&opt)
	}

	c, err := sc.NewConsumerFromClient(k.sc, opt.Queue, []string{topic})
	if err != nil {
		return nil, err
	}

	go func() {
		for {
			select {
			case sm := <-c.Messages():
				var m *broker.Message
				if err := json.Unmarshal(sm.Value, &m); err != nil {
					continue
				}

				handler(&publication{m: m, t: sm.Topic})
			}
		}
	}()

	return &subscriber{s: c, opts: opt}, nil
}

func (k *kBroker) String() string {
	return "kafka"
}

func NewBroker(addrs []string, opts ...broker.Option) broker.Broker {
	var options broker.Options

	for _, o := range opts {
		o(&options)
	}

	var cAddrs []string
	for _, addr := range addrs {
		if len(addr) == 0 {
			continue
		}
		cAddrs = append(cAddrs, addr)
	}
	if len(cAddrs) == 0 {
		cAddrs = []string{"127.0.0.1:9092"}
	}

	return &kBroker{
		addrs: cAddrs,
	}
}
