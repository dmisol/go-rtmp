package main

import (
	"log"
	"net/url"
	"path"
	"strings"
	"sync/atomic"

	"github.com/dmisol/go-rtmp"
	rtmplib "github.com/dmisol/go-rtmp"
	rtmpmsg "github.com/dmisol/go-rtmp/message"
	flvtag "github.com/yutopp/go-flv/tag"
)

const (
	chunkSize = 128
)

func NewSubOut(dest string) (s *SubOut, err error) {

	_, app, tcurl, pn := splitDest(dest)

	log.Println("got dest", dest)
	u, err := url.Parse(dest)
	if err != nil {
		s.Println("url parse", err)
		return
	}

	s = &SubOut{dest: dest}
	if s.client, err = rtmplib.Dial(u.Scheme, u.Host, &rtmp.ConnConfig{}); err != nil {
		s.Println("dial", err)
		return
	}
	s.Println("setup client conn to", dest)

	// TODO: youtube test, remove hardcoded...
	ncc := &rtmpmsg.NetConnectionConnect{
		Command: rtmpmsg.NetConnectionConnectCommand{
			FlashVer: "FMLE/3.0 (compatible; Lavf58.76.100)",
			App:      app, //"live2",
			Type:     "nonprivate",
			TCURL:    tcurl, //"rtmp://a.rtmp.youtube.com:1935/live2",
		},
	}
	if err = s.client.Connect(ncc); err != nil {
		s.Println("dial", err)
		return
	}

	if s.stream, err = s.client.CreateAndPublish(&rtmpmsg.NetStreamPublish{
		// ATTN! key is to be placed here
		// TODO: youtube test, remove hardcoded...
		PublishingName: pn, //"bb57-3kj8-uz95-rb6s-bkzz",
		PublishingType: "live",
	}, chunkSize); err != nil {
		s.Println("createStream()", err)
		return
	}

	s.eventCallback = onEventCallback(s.wr)
	return
}

func splitDest(dest string) (sch string, app string, tcurl string, pn string) {
	u, _ := url.Parse(dest)
	pth := u.Path
	dir := path.Dir(pth)

	sch = u.Scheme
	pn = path.Base(pth)
	app = strings.TrimPrefix(dir, "/")
	tcurl = sch + "://" + u.Host + dir

	return
}

type SubOut struct {
	dest   string
	client *rtmplib.ClientConn
	stream *rtmplib.Stream

	initialized   int64
	closed        bool
	lastTimestamp uint32
	eventCallback func(*flvtag.FlvTag) error
}

func (s *SubOut) wr(chunkStreamID int, ts uint32, rm rtmpmsg.Message) error {
	s.Println("ts", ts, rm.TypeID())
	return s.stream.Write(chunkStreamID, ts, rm)
}

func (s *SubOut) OnEvent(flv *flvtag.FlvTag) (err error) {
	if s.closed {
		return nil
	}

	if flv.Timestamp != 0 && s.lastTimestamp == 0 {
		s.lastTimestamp = flv.Timestamp
	}
	flv.Timestamp -= s.lastTimestamp

	return s.eventCallback(flv)
}

func (s *SubOut) IsInitialized() (res bool) {
	r := atomic.AddInt64(&s.initialized, 1)
	res = true
	if r == 1 {
		s.Println("init")
		res = false
	}
	return
}
func (s *SubOut) Close() (err error) {
	s.closed = true
	s.stream.Close()
	return s.client.Close()
}

func (s *SubOut) Println(i ...interface{}) {
	log.Println("[client to ", s.dest, "]", i)
}
