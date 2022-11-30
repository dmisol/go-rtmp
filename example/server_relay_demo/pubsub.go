package main

import (
	"bytes"
	"log"
	"sync"
	"sync/atomic"

	flvtag "github.com/yutopp/go-flv/tag"
)

type Pubsub struct {
	srv  *RelayService
	name string

	pub  *Pub
	subs []SubGeneric

	m sync.Mutex
}

func NewPubsub(srv *RelayService, name string) *Pubsub {
	return &Pubsub{
		srv:  srv,
		name: name,

		subs: make([]SubGeneric, 0),
	}
}

func (pb *Pubsub) Deregister() error {
	pb.m.Lock()
	defer pb.m.Unlock()

	for _, sub := range pb.subs {
		_ = sub.Close()
	}

	return pb.srv.RemovePubsub(pb.name)
}

func (pb *Pubsub) Pub() *Pub {
	pub := &Pub{
		pb: pb,
	}

	pb.pub = pub

	return pub
}

func (pb *Pubsub) NewSub(cb func(*flvtag.FlvTag) error) *SubIn {
	pb.m.Lock()
	defer pb.m.Unlock()

	sub := &SubIn{eventCallback: cb}

	// TODO: Implement more efficient resource management
	pb.subs = append(pb.subs, sub)

	return sub
}

func (pb *Pubsub) Append(s SubGeneric) {
	pb.m.Lock()
	defer pb.m.Unlock()
	pb.subs = append(pb.subs, s)
}

type Pub struct {
	pb *Pubsub

	avcSeqHeader *flvtag.FlvTag
	lastKeyFrame *flvtag.FlvTag

	scriptData *flvtag.FlvTag
}

func (p *Pub) Publish(flv *flvtag.FlvTag) error {
	switch flv.Data.(type) {
	case *flvtag.VideoData:
		d := flv.Data.(*flvtag.VideoData)
		if d.AVCPacketType == flvtag.AVCPacketTypeSequenceHeader {
			p.avcSeqHeader = flv
		}

		if d.FrameType == flvtag.FrameTypeKeyFrame {
			p.lastKeyFrame = flv
		}
	case *flvtag.ScriptData:
		log.Println("store scriptData")
		p.scriptData = flv
	}

	for _, sub := range p.pb.subs {
		if !sub.IsInitialized() {
			log.Println("init sequence")

			if p.scriptData != nil {
				log.Println(".. scriptData")
				if err := sub.OnEvent(cloneView(p.scriptData)); err != nil {
					log.Println("err", err)
				}
			} else {
				log.Println("scriptData is nil")
			}

			if p.avcSeqHeader != nil {
				log.Println(".. avcSeqHeader")
				if err := sub.OnEvent(cloneView(p.avcSeqHeader)); err != nil {
					log.Println("err", err)
				}
			} else {
				log.Println("avcSeqHeader is nil")
			}
			if p.lastKeyFrame != nil {
				log.Println(".. lastKeyFrame")
				if err := sub.OnEvent(cloneView(p.lastKeyFrame)); err != nil {
					log.Println("err,", err)
				}
			} else {
				log.Println("lastKeyFrame is nil")
			}
		}
		_ = sub.OnEvent(cloneView(flv))
	}
	return nil
}

func (p *Pub) Close() error {
	return p.pb.Deregister()
}

type SubGeneric interface {
	Close() error
	OnEvent(*flvtag.FlvTag) error
	IsInitialized() bool
}

type SubIn struct {
	initialized int64
	closed      bool

	lastTimestamp uint32
	eventCallback func(*flvtag.FlvTag) error
}

func (s *SubIn) OnEvent(flv *flvtag.FlvTag) error {
	if s.closed {
		return nil
	}

	if flv.Timestamp != 0 && s.lastTimestamp == 0 {
		s.lastTimestamp = flv.Timestamp
	}
	flv.Timestamp -= s.lastTimestamp

	return s.eventCallback(flv)
}

func (s *SubIn) Close() error {
	if s.closed {
		return nil
	}

	s.closed = true

	return nil
}

func (s *SubIn) IsInitialized() (res bool) {
	r := atomic.AddInt64(&s.initialized, 1)
	res = true
	if r == 1 {
		res = false
	}
	return
}

func cloneView(flv *flvtag.FlvTag) *flvtag.FlvTag {
	// Need to clone the view because Binary data will be consumed
	v := *flv

	switch flv.Data.(type) {
	case *flvtag.AudioData:
		dCloned := *v.Data.(*flvtag.AudioData)
		v.Data = &dCloned

		dCloned.Data = bytes.NewBuffer(dCloned.Data.(*bytes.Buffer).Bytes())

	case *flvtag.VideoData:
		dCloned := *v.Data.(*flvtag.VideoData)
		v.Data = &dCloned

		dCloned.Data = bytes.NewBuffer(dCloned.Data.(*bytes.Buffer).Bytes())

	case *flvtag.ScriptData:
		dCloned := *v.Data.(*flvtag.ScriptData)
		v.Data = &dCloned

	default:
		panic("unreachable")
	}

	return &v
}
