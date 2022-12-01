package main

import (
	"bytes"

	rtmpmsg "github.com/dmisol/go-rtmp/message"
	flvtag "github.com/yutopp/go-flv/tag"
)

type msgWriter func(int, uint32, rtmpmsg.Message) error

func onEventCallback(wr msgWriter) func(flv *flvtag.FlvTag) error {
	return func(flv *flvtag.FlvTag) error {
		buf := new(bytes.Buffer)

		switch flv.Data.(type) {
		case *flvtag.AudioData:
			//log.Println("audio")
			d := flv.Data.(*flvtag.AudioData)

			// Consume flv payloads (d)
			if err := flvtag.EncodeAudioData(buf, d); err != nil {
				return err
			}

			msg := &rtmpmsg.AudioMessage{
				Payload: buf,
			}
			return wr(5, flv.Timestamp, msg)
		case *flvtag.VideoData:
			//log.Println("video")
			d := flv.Data.(*flvtag.VideoData)

			// Consume flv payloads (d)
			if err := flvtag.EncodeVideoData(buf, d); err != nil {
				return err
			}

			msg := &rtmpmsg.VideoMessage{
				Payload: buf,
			}
			return wr(6, flv.Timestamp, msg)
		case *flvtag.ScriptData:
			//log.Println("script")
			d := flv.Data.(*flvtag.ScriptData)

			// Consume flv payloads (d)
			if err := flvtag.EncodeScriptData(buf, d); err != nil {
				return err
			}

			// TODO: hide these implementation
			amdBuf := new(bytes.Buffer)
			amfEnc := rtmpmsg.NewAMFEncoder(amdBuf, rtmpmsg.EncodingTypeAMF0)
			if err := rtmpmsg.EncodeBodyAnyValues(amfEnc, &rtmpmsg.NetStreamSetDataFrame{
				Payload: buf.Bytes(),
			}); err != nil {
				return err
			}

			msg := &rtmpmsg.DataMessage{
				Name:     "@setDataFrame", // TODO: fix
				Encoding: rtmpmsg.EncodingTypeAMF0,
				Body:     amdBuf,
			}
			return wr(8, flv.Timestamp, msg)
		default:
			panic("unreachable")
		}
	}
}
