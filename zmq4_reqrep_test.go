// Copyright 2018 The go-zeromq Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package zmq4_test

import (
	"context"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/go-zeromq/zmq4"
	"github.com/go-zeromq/zmq4/zmtp"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
)

var (
	reqreps = []testCaseReqRep{
		{
			name:     "tcp-req-rep",
			endpoint: must(EndPoint("tcp")),
			req:      zmq4.NewReq(bkg),
			rep:      zmq4.NewRep(bkg),
		},
		{
			name:     "ipc-req-rep",
			endpoint: "ipc://ipc-req-rep",
			req:      zmq4.NewReq(bkg),
			rep:      zmq4.NewRep(bkg),
		},
	}
)

type testCaseReqRep struct {
	name     string
	skip     bool
	endpoint string
	req      zmq4.Socket
	rep      zmq4.Socket
}

func TestReqRep(t *testing.T) {
	var (
		reqName = zmtp.NewMsgString("NAME")
		reqLang = zmtp.NewMsgString("LANG")
		reqQuit = zmtp.NewMsgString("QUIT")
		repName = zmtp.NewMsgString("zmq4")
		repLang = zmtp.NewMsgString("Go")
		repQuit = zmtp.NewMsgString("bye")
	)

	for _, tc := range reqreps {
		t.Run(tc.name, func(t *testing.T) {
			if tc.skip {
				t.Skipf(tc.name)
			}

			// FIXME(sbinet): we should probably do this at the zmq4.Socket.Close level
			if strings.HasPrefix(tc.endpoint, "ipc://") {
				defer os.Remove(tc.endpoint[len("ipc://"):])
			}

			ep := tc.endpoint

			ctx, timeout := context.WithTimeout(context.Background(), 20*time.Second)
			defer timeout()

			defer tc.req.Close()
			defer tc.rep.Close()

			grp, ctx := errgroup.WithContext(ctx)
			grp.Go(func() error {

				err := tc.rep.Listen(ep)
				if err != nil {
					return errors.Wrapf(err, "could not listen")
				}

				loop := true
				for loop {
					msg, err := tc.rep.Recv()
					if err != nil {
						return errors.Wrapf(err, "could not recv REQ message")
					}
					var rep zmtp.Msg
					switch string(msg.Frames[0]) {
					case "NAME":
						rep = repName
					case "LANG":
						rep = repLang
					case "QUIT":
						rep = repQuit
						loop = false
					}

					err = tc.rep.Send(rep)
					if err != nil {
						return errors.Wrapf(err, "could not send REP message to %v", msg)
					}
				}

				return err
			})
			grp.Go(func() error {

				err := tc.req.Dial(ep)
				if err != nil {
					return errors.Wrapf(err, "could not dial")
				}

				for _, msg := range []struct {
					req zmtp.Msg
					rep zmtp.Msg
				}{
					{reqName, repName},
					{reqLang, repLang},
					{reqQuit, repQuit},
				} {
					err = tc.req.Send(msg.req)
					if err != nil {
						return errors.Wrapf(err, "could not send REQ message %v", msg.req)
					}
					rep, err := tc.req.Recv()
					if err != nil {
						return errors.Wrapf(err, "could not recv REP message %v", msg.req)
					}

					if got, want := rep, msg.rep; !reflect.DeepEqual(got, want) {
						return errors.Wrapf(err, "got = %v, want= %v", got, want)
					}
				}

				return err
			})
			if err := grp.Wait(); err != nil {
				t.Fatal(err)
			}
		})
	}
}
