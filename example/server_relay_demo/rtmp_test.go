package main

import (
	"context"
	"fmt"
	"log"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

const (
	ffplaySrvAddr = "rtmp://localhost:12345/stream"
	ffplaySrvArgs = "-f flv -listen 1 -i " + ffplaySrvAddr
	ytRtmp        = "rtmp://localhost:1935/appname/clienttest"
)

func TestSrv(t *testing.T) {

	//t.Skip()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	id := uuid.NewString()
	streamUrl := fmt.Sprintf("rtmp://localhost/appname/%s", id)
	log.Println(streamUrl)

	// start server
	srv := NewRelayServer()
	// .. and start a relay
	relay := srv.AddPipeline(ctx, id)
	log.Println(relay)
	// publish to relay

	//args := fmt.Sprintf("-re -y -i testdata/movie.mp4 -c:a copy -ac 1 -ar 44100 -b:a 96k -vcodec copy -tune zerolatency -f flv %s", streamUrl)
	args := `-f rawvideo -f pulse -ac 2 -i default -c:v libx264 -preset fast -pix_fmt yuv420p -s 1280x800 -c:a aac -b:a 160k -ar 44100 -threads 0 -f flv rtmp://localhost/appname/` + id
	a := strings.Split(args, " ")
	cmd := exec.CommandContext(ctx, "ffmpeg", a...)

	go func() {
		defer log.Println("test ffmpeg publisher ended")

		if err := cmd.Run(); err != nil {
			t.Errorf(err.Error())
		}
		log.Println("file ended", args)
	}()

	time.Sleep(2 * time.Second)

	/*
		go func() {
			defer log.Println("ffplay server ended")

			args = ffplaySrvArgs
			a = strings.Split(args, " ")
			cmd = exec.CommandContext(ctx, "ffplay", a...)
			if err := cmd.Run(); err != nil {
				t.Errorf(err.Error())
			}
			log.Println("ffplay stopped")
		}()
	*/

	time.Sleep(3 * time.Second)
	relay.AddSink(ytRtmp) //ffplaySrvAddr)

	time.Sleep(50 * time.Second)
	relay.RemoveSink(ytRtmp) //ffplaySrvAddr)

	<-ctx.Done()
}
