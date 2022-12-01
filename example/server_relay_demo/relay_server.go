package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"os/exec"
	"strings"
	"sync"

	//	log "github.com/sirupsen/logrus"
	"github.com/dmisol/go-rtmp"
)

const useFfmpeg = false

type RelayServer struct {
	mu sync.Mutex

	relayService *RelayService
	cefs         map[string]*Relay
}

func NewRelayServer() *RelayServer {
	rs := &RelayServer{cefs: map[string]*Relay{}}
	//	log.SetLevel(log.WarnLevel)

	ch := make(chan bool)
	go rs.run(ch)
	<-ch
	return rs
}
func (r *RelayServer) run(ch chan bool) {
	tcpAddr, err := net.ResolveTCPAddr("tcp", ":1935")
	if err != nil {
		log.Panicf("Failed: %+v", err)
	}

	listener, err := net.ListenTCP("tcp", tcpAddr)
	if err != nil {
		log.Panicf("Failed: %+v", err)
	}

	r.relayService = NewRelayService(r)
	ch <- true

	log.Println("starting relay Server")

	srv := rtmp.NewServer(&rtmp.ServerConfig{
		OnConnect: func(conn net.Conn) (io.ReadWriteCloser, *rtmp.ConnConfig) {
			log.Println("onConnect()", conn.LocalAddr(), conn.RemoteAddr())
			//			l := log.StandardLogger()
			//l.SetLevel(logrus.DebugLevel)

			h := &Handler{
				relayService: r.relayService,
			}

			return conn, &rtmp.ConnConfig{
				Handler: h,

				ControlState: rtmp.StreamControlStateConfig{
					DefaultBandwidthWindowSize: 6 * 1024 * 1024 / 8,
				},
				//
				//				Logger: l,
			}
		},
	})
	if err := srv.Serve(listener); err != nil {
		log.Panicf("Failed: %+v", err)
	}
}

func (rs *RelayServer) AddPipeline(ctx context.Context, id string) (r *Relay) {
	r = &Relay{
		ctx:       ctx,
		sinkStops: make(map[string]context.CancelFunc),
		id:        id,
		service:   rs.relayService,
	}

	rs.mu.Lock()
	defer rs.mu.Unlock()

	rs.cefs[id] = r

	return
}

func (rs *RelayServer) RemovePipeline(id string) {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	if r, ok := rs.cefs[id]; ok {
		r.removaAll()
	}

	delete(rs.cefs, id)

}

type Relay struct {
	mu      sync.Mutex
	id      string
	service *RelayService

	ctx       context.Context
	sinkStops map[string]context.CancelFunc
}

func (r *Relay) AddSink(dest string) {
	if useFfmpeg {
		r.mu.Lock()
		defer r.mu.Unlock()

		r.sinkStops[dest] = r.addFfmpegSink(dest)
	} else {
		c, err := r.addDirectSink(dest)
		if err != nil {
			r.Println("adding sink", err)
			return
		}

		r.mu.Lock()
		defer r.mu.Unlock()

		r.sinkStops[dest] = c
	}

	r.Println("add", dest)
}

func (r *Relay) addDirectSink(dest string) (cancel func(), err error) {
	// todo: setup client conn to dest
	// and add it to Pubsub's subscribers directly,
	// as SubOut matching SubGeneric

	ctx, cancel := context.WithCancel(context.Background())

	// todo: start go-routine here?
	s, err := NewSubOut(dest)
	if err != nil {
		r.Println("client conn", err)
		cancel() // need to release ctx
		return
	}

	r.Println("locking to append")
	r.mu.Lock()
	defer r.mu.Unlock()

	var ps *Pubsub
	if ps, err = r.service.GetPubsub(r.id); err != nil {
		r.Println("can't get pubsub", err)
		return
	}
	ps.Append(s)

	r.Println("appended")

	go func() {
		<-ctx.Done()
		r.Println("client closed by ctx", dest)
		s.Close()
	}()
	return
}

func (r *Relay) RemoveSink(dest string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if cancel, ok := r.sinkStops[dest]; ok {
		cancel()
	}

	delete(r.sinkStops, dest)
	r.Println("remove", dest)
}

func (r *Relay) addFfmpegSink(dest string) (cancel context.CancelFunc) {
	ctx, c := context.WithCancel(r.ctx)
	cancel = c

	args := fmt.Sprintf("-i rtmp://localhost/appname/%s -c:a copy -vcodec copy -tune zerolatency -f flv %s", r.id, dest)
	r.Println("ffmpeg", args)

	a := strings.Split(args, " ")
	cmd := exec.CommandContext(ctx, "ffmpeg", a...)

	go func() {
		defer func() {
			r.Println("done", dest)
		}()
		cmd.Run()
	}()
	return
}

func (r *Relay) removaAll() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.Println("removing all sinks")
	for _, v := range r.sinkStops {
		v()
	}
}

func (r *Relay) Println(i ...interface{}) {
	log.Println("relay", r.id, i)
}
