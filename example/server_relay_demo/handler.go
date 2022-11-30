package main

import (
	"bytes"
	"context"
	"io"
	"log"

	rtmp "github.com/dmisol/go-rtmp"
	rtmpmsg "github.com/dmisol/go-rtmp/message"
	"github.com/pkg/errors"
	flvtag "github.com/yutopp/go-flv/tag"
)

var _ rtmp.Handler = (*Handler)(nil)

// Handler An RTMP connection handler
type Handler struct {
	rtmp.DefaultHandler
	relayService *RelayService

	//
	conn *rtmp.Conn

	//
	pub *Pub
	sub *SubIn
}

func (h *Handler) OnServe(conn *rtmp.Conn) {
	h.conn = conn
}

func (h *Handler) OnConnect(timestamp uint32, cmd *rtmpmsg.NetConnectionConnect) error {
	log.Println("handler onConnect", cmd)
	args, _ := cmd.ToArgs(rtmpmsg.EncodingTypeAMF3) // arg is ignored
	log.Println("handler onConnect:", args)
	arg := args[0].(rtmpmsg.NetConnectionConnectCommand)
	log.Println(arg.FlashVer)

	return nil
}

func (h *Handler) OnCreateStream(timestamp uint32, cmd *rtmpmsg.NetConnectionCreateStream) error {
	return nil
}

func (h *Handler) OnPublish(_ *rtmp.StreamContext, timestamp uint32, cmd *rtmpmsg.NetStreamPublish) error {
	log.Println("handler onPublish", cmd)

	if h.sub != nil {
		return errors.New("Cannot publish to this stream")
	}

	// (example) Reject a connection when PublishingName is empty
	if cmd.PublishingName == "" {
		return errors.New("PublishingName is empty")
	}

	pubsub, err := h.relayService.NewPubsub(cmd.PublishingName)
	if err != nil {
		return errors.Wrap(err, "Failed to create pubsub")
	}

	pub := pubsub.Pub()

	h.pub = pub

	return nil
}

func (h *Handler) OnPlay(ctx *rtmp.StreamContext, timestamp uint32, cmd *rtmpmsg.NetStreamPlay) (err error) {
	//log.Println("onPlay()")
	if h.sub != nil {
		err = errors.New("Cannot play on this stream")
		log.Println(err)
		return
	}

	pubsub, err := h.relayService.GetPubsub(cmd.StreamName)
	if err != nil {
		err = errors.Wrap(err, "Failed to get pubsub")
		log.Println(err)
		return
	}

	//sub := pubsub.NewSub(onEventCallback(h.conn, ctx.StreamID))
	sub := pubsub.NewSub(onEventCallback(func(chunkStreamID int, ts uint32, rm rtmpmsg.Message) error {
		ch := &rtmp.ChunkMessage{
			StreamID: ctx.StreamID,
			Message:  rm,
		}
		return h.conn.Write(context.Background(), chunkStreamID, ts, ch)
	}))

	h.sub = sub
	//log.Println("onPlay() succeed")
	return nil
}

func (h *Handler) OnSetDataFrame(timestamp uint32, data *rtmpmsg.NetStreamSetDataFrame) error {
	log.Println("onSetDataFrame()")

	r := bytes.NewReader(data.Payload)

	var script flvtag.ScriptData
	if err := flvtag.DecodeScriptData(r, &script); err != nil {
		log.Printf("Failed to decode script data: Err = %+v", err)
		return nil // ignore
	}

	_ = h.pub.Publish(&flvtag.FlvTag{
		TagType:   flvtag.TagTypeScriptData,
		Timestamp: timestamp,
		Data:      &script,
	})

	return nil
}

func (h *Handler) OnAudio(timestamp uint32, payload io.Reader) error {
	var audio flvtag.AudioData
	if err := flvtag.DecodeAudioData(payload, &audio); err != nil {
		return err
	}

	flvBody := new(bytes.Buffer)
	if _, err := io.Copy(flvBody, audio.Data); err != nil {
		return err
	}
	audio.Data = flvBody

	_ = h.pub.Publish(&flvtag.FlvTag{
		TagType:   flvtag.TagTypeAudio,
		Timestamp: timestamp,
		Data:      &audio,
	})

	return nil
}

func (h *Handler) OnVideo(timestamp uint32, payload io.Reader) error {
	var video flvtag.VideoData
	if err := flvtag.DecodeVideoData(payload, &video); err != nil {
		return err
	}

	// Need deep copy because payload will be recycled
	flvBody := new(bytes.Buffer)
	if _, err := io.Copy(flvBody, video.Data); err != nil {
		return err
	}
	video.Data = flvBody

	_ = h.pub.Publish(&flvtag.FlvTag{
		TagType:   flvtag.TagTypeVideo,
		Timestamp: timestamp,
		Data:      &video,
	})

	return nil
}

func (h *Handler) OnClose() {
	if h.pub != nil {
		_ = h.pub.Close()
	}

	if h.sub != nil {
		_ = h.sub.Close()
	}
}
