//
// Copyright (c) 2018- yutopp (yutopp@gmail.com)
//
// Distributed under the Boost Software License, Version 1.0. (See accompanying
// file LICENSE_1_0.txt or copy at  https://www.boost.org/LICENSE_1_0.txt)
//

package rtmp

import (
	"log"
	"net"
	"sync"

	"github.com/pkg/errors"

	"github.com/dmisol/go-rtmp/handshake"
	"github.com/dmisol/go-rtmp/message"
)

// ClientConn A wrapper of a connection. It prorives client-side specific features.
type ClientConn struct {
	conn    *Conn
	lastErr error
	m       sync.RWMutex
}

func newClientConnWithSetup(c net.Conn, config *ConnConfig) (*ClientConn, error) {
	conn := newConn(c, config)

	if err := handshake.HandshakeWithServer(conn.rwc, conn.rwc, &handshake.Config{
		SkipHandshakeVerification: conn.config.SkipHandshakeVerification,
	}); err != nil {
		return nil, errors.Wrap(err, "Failed to handshake")
	}

	ctrlStream, err := conn.streams.Create(ControlStreamID, nil)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create control stream")
	}
	ctrlStream.handler.ChangeState(streamStateClientNotConnected)

	conn.streamer.controlStreamWriter = ctrlStream.Write

	cc := &ClientConn{
		conn: conn,
	}
	go cc.startHandleMessageLoop()

	return cc, nil
}

func (cc *ClientConn) Close() error {
	return cc.conn.Close()
}

func (cc *ClientConn) LastError() error {
	cc.m.RLock()
	defer cc.m.RUnlock()

	return cc.lastErr
}

func (cc *ClientConn) Connect(body *message.NetConnectionConnect) error {
	if err := cc.controllable(); err != nil {
		return err
	}

	stream, err := cc.conn.streams.At(ControlStreamID)
	if err != nil {
		return err
	}

	result, err := stream.Connect(body)
	if err != nil {
		return err // TODO: wrap an error
	}

	// TODO: check result
	_ = result

	return nil
}

func (cc *ClientConn) CreateStream(body *message.NetConnectionCreateStream, chunkSize uint32) (*Stream, error) {
	if err := cc.controllable(); err != nil {
		return nil, err
	}

	stream, err := cc.conn.streams.At(ControlStreamID)
	if err != nil {
		return nil, err
	}

	result, err := stream.CreateStream(body, chunkSize)
	if err != nil {
		return nil, err // TODO: wrap an error
	}

	// TODO: check result

	newStream, err := cc.conn.streams.Create(result.StreamID, nil)
	if err != nil {
		return nil, err
	}

	return newStream, nil
}

func (cc *ClientConn) CreateAndPublish(body *message.NetStreamPublish, chunkSize uint32, cb func()) (*Stream, error) {
	if err := cc.controllable(); err != nil {
		return nil, err
	}

	stream, err := cc.conn.streams.At(ControlStreamID)
	if err != nil {
		return nil, err
	}

	rel := &message.NetConnectionReleaseStream{
		StreamName: body.PublishingName,
	}
	stream.WriteCommandMessage(3, 0, "releaseStream", 2, rel)

	fcp := &message.NetStreamFCPublish{
		StreamName: body.PublishingName,
	}

	stream.WriteCommandMessage(3, 0, "FCPublish", 3, fcp)

	result, err := stream.CreateStream(nil, chunkSize)
	if err != nil {
		return nil, err // TODO: wrap an error
	}

	// TODO: check result

	newStream, err := cc.conn.streams.Create(result.StreamID, cb)
	if err != nil {
		return nil, err
	}

	if err = newStream.Publish(body); err != nil {
		log.Println("publish", err)
		return newStream, err
	}

	return newStream, nil
}

func (cc *ClientConn) startHandleMessageLoop() {
	if err := cc.conn.handleMessageLoop(); err != nil {
		cc.setLastError(err)
	}
}

func (cc *ClientConn) setLastError(err error) {
	cc.m.Lock()
	defer cc.m.Unlock()

	cc.lastErr = err
}

func (cc *ClientConn) controllable() error {
	err := cc.LastError()
	return errors.Wrap(err, "Client is in error state")
}
