//go:build ignore

// Command gen renders the animated SVG diagrams used by the docs.
//
// Run it from the repository root after changing a diagram:
//
//	go run docs/diagrams/gen.go
//
// Use -at to render a static frame at a point in time, e.g. to review a
// diagram with an SVG viewer that does not play CSS animations:
//
//	go run docs/diagrams/gen.go -at 2.5 -out /tmp/frames
//
// The colors follow the Catppuccin Frappé palette of the docs theme. Blue
// tokens are messages going in. Green tokens are handled messages, acks and
// replies. Purple tokens are transformed messages. Red tokens are errors, naks
// and dropped messages.
package main

import (
	"flag"
	"fmt"
	"html"
	"log"
	"math"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

const (
	speed  = 150.0 // token speed in px per second
	fade   = 0.25  // fade in/out duration in seconds
	hold   = 1.5   // pause after the last move before the loop restarts
	tokenH = 30.0
	hopH   = 26.0 // height of the hop over waiting tokens
	gap    = 6.0  // space between waiting tokens
)

const style = `<style>
text{font:600 14px ui-monospace,SFMono-Regular,Menlo,monospace;text-anchor:middle;dominant-baseline:central;fill:#303446}
.op{fill:#c6d0f5}
.sub{font:500 12px ui-monospace,SFMono-Regular,Menlo,monospace;fill:#a5adce}
.lbl{font:500 13px ui-sans-serif,system-ui,sans-serif;fill:#838ba7}
.s{text-anchor:start}.e{text-anchor:end}
.in{fill:#8caaee}.mid{fill:#ca9ee6}.out{fill:#a6d189}.err{fill:#e78284}
.box{fill:#414559;stroke:#838ba7;stroke-width:1.5}
.wire{stroke:#838ba7;stroke-width:1.5;stroke-dasharray:4 4;fill:none}
.zone{fill:none;stroke:#626880;stroke-width:1.5;stroke-dasharray:8 5}
@media (prefers-reduced-motion:reduce){g{animation:none!important}.k{opacity:1}.h{opacity:0}}
</style>
`

func main() {
	at := flag.Float64("at", -1, "render a static frame at this second instead of the animation")
	out := flag.String("out", "docs/diagrams", "output directory")
	flag.Parse()

	for _, d := range diagrams() {
		name := filepath.Join(*out, d.name+".svg")
		if err := os.WriteFile(name, []byte(d.render(*at)), 0o600); err != nil {
			log.Fatal(err)
		}
	}
}

func diagrams() []*diagram {
	return []*diagram{
		publishSubscribe(),
		channelVsNATS(),
		readiness(),
		fireAndForget(),
		atLeastOnce(),
		manualAck(),
		dedup(),
		pullConsumer(),
		streamProcessor(),
		headers(),
		httpIngest(),
		requestReply(),
		queueGroups(),
		middlewareChain(),
		pipeNew(),
		pipeMap(),
		pipeFlatMap(),
		retryPublisher(),
		goflowIntegration(),
		goflowFanOut(),
		goflowDistinct(),
		goflowFanIn(),
		goflowProcess(),
		streamTee(),
		streamRoundRobin(),
		streamFanIn(),
	}
}

// ~ Diagrams

// linear is the basic flow: messages queue left, pass every box and come out
// right. outs[i] is the class of the output for ins[i].
func linear(name, alt string, boxes []string, ins, outs []string) *diagram {
	return chain(name, alt, boxes, ins, outs, nil, "")
}

// chain is linear, but drop maps an index of ins to the index of the box that
// drops it. A dropped message turns red and stays below that box, marked with
// dropLabel.
func chain(name, alt string, boxes []string, ins, outs []string, drop map[int]int, dropLabel string) *diagram {
	const y, boxW, boxGap = 55.0, 130.0, 60.0

	h := 110.0
	if len(drop) > 0 {
		h = 165
	}
	d := newDiagram(name, alt, 0, h)
	x0, gate := d.queue(ins, 20)
	gate += boxGap

	cx := make([]float64, len(boxes))
	for i, b := range boxes {
		cx[i] = gate + boxW/2 + float64(i)*(boxW+boxGap)
		d.box(cx[i], y, boxW, 50, b)
	}

	exit := cx[len(cx)-1] + boxW/2
	d.wire(gate-boxGap+10, y, gate, y)
	for i := 1; i < len(cx); i++ {
		d.wire(cx[i-1]+boxW/2, y, cx[i]-boxW/2, y)
	}
	d.wire(exit, y, exit+boxGap, y)

	var kept []string
	for i, m := range ins {
		if _, ok := drop[i]; !ok {
			kept = append(kept, m)
		}
	}
	slots := d.slots(kept, exit+boxGap+10)
	d.w = slots[len(slots)-1] + 40

	n := 0
	for i, m := range ins {
		t := 0.4 + float64(i)*1.6
		k := d.tok("in", m, x0[i], y, true).keep().until(t).enter(x0[i], gate, cx[0], y, i < len(ins)-1)
		if b, ok := drop[i]; ok {
			for _, x := range cx[1 : b+1] {
				k.to(x, y)
			}
			done := k.now()
			k.hide()
			d.text(cx[b], y+80, "", dropLabel)
			d.tok("err", m, cx[b], y, false).until(done).show().to(cx[b], y+50).keep()

			continue
		}
		for _, x := range cx[1:] {
			k.to(x, y)
		}
		done := k.now()
		k.hide()
		d.tok(outs[i], m, cx[len(cx)-1], y, false).until(done).show().leave(exit, slots[n], y, n > 0).keep()
		n++
	}

	return d
}

func publishSubscribe() *diagram {
	return linear("publish-subscribe",
		"The publisher sends m1, m2, m3 through the transport on orders.created; the subscriber's handler receives each one",
		[]string{"Publisher", "Transport\norders.created", "Handler"},
		[]string{"m1", "m2", "m3"}, []string{"out", "out", "out"})
}

func channelVsNATS() *diagram {
	const boxW = 120.0

	d := newDiagram("channel-vs-nats",
		"With channels, m1 stays a Go value inside one process. With NATS, the publisher encodes m1, the NATS server delivers the bytes to every subscribing service, and each service decodes m1", 730, 310)
	pub, srv, h := 150.0, 360.0, 570.0

	// channel: one process, in memory.
	y := 65.0
	d.text(20, 18, "s", "channel: one process, values in memory")
	d.zone(20, 33, 690, 64)
	d.wire(70, y, pub-boxW/2, y)
	d.wire(pub+boxW/2, y, srv-boxW/2, y)
	d.wire(srv+boxW/2, y, h-boxW/2, y)
	d.wire(h+boxW/2, y, h+boxW/2+20, y)
	d.box(pub, y, boxW, 44, "Publisher")
	d.box(srv, y, boxW, 44, "Bus\nin memory")
	d.box(h, y, boxW, 44, "Handler")

	k := d.tok("in", "m1", 50, y, true).keep().until(0.4).to(pub, y).to(srv, y).to(h, y)
	done := k.now()
	k.hide()
	d.tok("out", "m1", h, y, false).until(done).show().to(h+boxW/2+40, y).keep()

	// NATS: services in separate processes, bytes over the network.
	mid, split := 225.0, 455.0
	rows := []float64{185, 265}
	d.text(20, 138, "s", "NATS: any number of processes, bytes over the network")
	d.zone(20, mid-32, 210, 64)
	d.wire(70, mid, pub-boxW/2, mid)
	d.wire(pub+boxW/2, mid, srv-boxW/2, mid)
	d.box(pub, mid, boxW, 44, "Publisher\nservice A")
	d.box(srv, mid, boxW, 50, "NATS server\norders")

	for i, ry := range rows {
		d.zone(480, ry-32, 230, 64)
		d.wire(srv+boxW/2, mid, split, mid, split, ry, h-boxW/2, ry)
		d.wire(h+boxW/2, ry, h+boxW/2+20, ry)
		d.box(h, ry, boxW, 44, "Handler\n"+[]string{"service B", "service C"}[i])
	}

	k = d.tok("in", "m1", 50, mid, true).keep().until(0.4).to(pub, mid)
	done = k.now()
	k.hide()
	k = d.tok("mid", "{…}", pub, mid, false).until(done).show().to(srv, mid)
	done = k.now()
	k.hide()

	for _, ry := range rows {
		k = d.tok("mid", "{…}", srv, mid, false).until(done).show().to(split, mid).to(split, ry).to(h, ry)
		arrive := k.now()
		k.hide()
		d.tok("out", "m1", h, ry, false).until(arrive).show().to(h+boxW/2+40, ry).keep()
	}

	return d
}

func fireAndForget() *diagram {
	return linear("fire-and-forget",
		"NATS core delivers m1, m2, m3 once; the handler fails on m2 and m2 is not redelivered",
		[]string{"Publisher", "NATS core\nevents.>", "Handler"},
		[]string{"m1", "m2", "m3"}, []string{"out", "err", "out"})
}

func readiness() *diagram {
	const boxW = 130.0

	d := newDiagram("readiness",
		"Publishing right after go Subscribe drops m1; publishing after ready() delivers it", 640, 210)
	rows := []struct {
		y           float64
		label, hint string
	}{
		{65, "go sub.Subscribe(…) then Publish", "subscribing…"},
		{165, "<-ready then Publish", "ready() called"},
	}
	cx := []float64{140, 310, 480}

	for i, r := range rows {
		d.text(20, r.y-40, "s", r.label)
		d.wire(60, r.y, cx[0]-boxW/2, r.y)
		d.wire(cx[0]+boxW/2, r.y, cx[1]-boxW/2, r.y)
		d.wire(cx[1]+boxW/2, r.y, cx[2]-boxW/2, r.y)
		d.wire(cx[2]+boxW/2, r.y, cx[2]+boxW/2+30, r.y)
		d.box(cx[0], r.y, boxW, 50, "Publish")
		d.box(cx[1], r.y, boxW, 50, "NATS core")
		d.box(cx[2], r.y, boxW, 50, "Handler\n"+r.hint)

		t := 0.4 + float64(i)*4
		k := d.tok("in", "m1", 40, r.y, true).keep().until(t).to(cx[0], r.y).to(cx[1], r.y)
		done := k.now()
		k.hide()

		if i == 0 {
			drop := (cx[1] + cx[2]) / 2
			d.text(drop, r.y+28, "", "dropped")
			d.tok("err", "m1", cx[1], r.y, false).until(done).show().to(drop, r.y).keep()

			continue
		}

		k.to(cx[2], r.y)
		done = k.now()
		d.tok("out", "m1", cx[2], r.y, false).until(done).show().to(cx[2]+boxW/2+60, r.y).keep()
	}

	return d
}

func atLeastOnce() *diagram {
	return ackFlow("at-least-once",
		"The handler acks m1; it fails on m2, JetStream gets a nak and redelivers m2, then the handler acks it",
		"Handler", "ack / nak",
		[]string{"m1", "m2"}, [][]string{{"ack"}, {"nak", "ack"}})
}

func manualAck() *diagram {
	return ackFlow("manual-ack",
		"With WithManualAck the handler acks m1; it naks m2 with a 5s delay, JetStream redelivers m2 after the delay and the handler acks it; it terminates m3, which is not redelivered",
		"Handler\nWithManualAck", "ack / nak / term",
		[]string{"m1", "m2", "m3"}, [][]string{{"ack"}, {"nak 5s", "ack"}, {"term"}})
}

// ackFlow delivers each of ins from JetStream to the handler, which answers
// with replies[i] in turn. Every reply before the last redelivers the message;
// a reply with a delay ("nak 5s") waits before the redelivery. The last reply
// is "ack" (handled) or "term" (dropped).
func ackFlow(name, alt, handler, label string, ins []string, replies [][]string) *diagram {
	const y, lane = 55.0, 108.0

	d := newDiagram(name, alt, 0, 160)
	x0, gate := d.queue(ins, 20)
	js, h := gate+30+65, gate+30+65+200

	d.wire(gate, y, js-65, y)
	d.wire(js+65, y, h-65, y)
	d.wire(h+65, y, h+95, y)
	d.wire(h, y+25, h, lane, js, lane, js, y+25)
	d.text((js+h)/2, lane+30, "", label)
	d.box(js, y, 130, 50, "JetStream\nEVENTS")
	d.box(h, y, 130, 50, handler)
	slots := d.slots(ins, h+105)
	d.w = slots[len(slots)-1] + 40

	back := func(class, text string, t float64) float64 {
		k := d.tok(class, text, h, y, false).until(t).show().to(h, lane).to(js, lane).to(js, y)
		done := k.now()
		k.hide()

		return done
	}

	t := 0.4
	for i, m := range ins {
		k := d.tok("in", m, x0[i], y, true).keep().until(t).enter(x0[i], gate, js, y, i < len(ins)-1).wait(0.3).to(h, y)
		done := k.now()
		k.hide()

		rs := replies[i]
		for _, r := range rs[:len(rs)-1] {
			done = back("err", r, done+0.2)
			if strings.Contains(r, " ") {
				done += 0.8
			}
			k = d.tok("in", m, js, y, false).until(done+0.2).show().to(h, y)
			done = k.now()
			k.hide()
		}

		class := "out"
		if rs[len(rs)-1] != "ack" {
			class = "err"
		}
		back(class, rs[len(rs)-1], done+0.2)
		d.tok(class, m, h, y, false).until(done+0.2).show().leave(h+65, slots[i], y, i > 0).keep()
		t = done
	}

	return d
}

func dedup() *diagram {
	return chain("dedup",
		"The publisher sends m1, retries m1 with the same message ID, then sends m2; JetStream drops the second m1 as a duplicate, so the handler gets m1 and m2 once",
		[]string{"Publisher\nWithMessageID", "JetStream\nNats-Msg-Id", "Handler"},
		[]string{"m1", "m1", "m2"}, []string{"out", "", "out"}, map[int]int{1: 1}, "duplicate ID")
}

func streamProcessor() *diagram {
	return chain("stream-processor",
		"ToStream feeds e1, e2, e1, e3 into Distinct, which drops the second e1; Peek logs each remaining event and Process(10) handles e1, e2, e3",
		[]string{"Distinct\nby ID", "Peek\nlog", "Process(10)\nhandle"},
		[]string{"e1", "e2", "e1", "e3"}, []string{"out", "out", "", "out"}, map[int]int{2: 0}, "duplicate")
}

func pullConsumer() *diagram {
	const y, lane = 55.0, 108.0

	d := newDiagram("pull-consumer",
		"The subscriber pulls 2 messages from JetStream, the handler processes t1 and t2, then the subscriber pulls the next 2 and the handler processes t3 and t4", 0, 160)
	ins := []string{"t1", "t2", "t3", "t4"}
	x0, gate := d.queue(ins, 20)
	js := gate + 30 + 65
	c := js + 65 + 60 + 80
	h := c + 80 + 60 + 65

	d.wire(gate, y, js-65, y)
	d.wire(js+65, y, c-80, y)
	d.wire(c+80, y, h-65, y)
	d.wire(h+65, y, h+95, y)
	d.wire(c, y+25, c, lane, js, lane, js, y+25)
	d.text((js+c)/2, lane+30, "", "pull 2 when idle")
	d.box(js, y, 130, 50, "JetStream\nTASKS")
	d.box(c, y, 160, 50, "Subscriber\npulls 2 at a time")
	d.box(h, y, 130, 50, "Handler\nRetryAck")
	slots := d.slots(ins, h+105)
	d.w = slots[len(slots)-1] + 40

	t, free := 0.4, 0.0
	for b := range 2 {
		k := d.tok("out", "pull", c, y, false).until(t).show().to(c, lane).to(js, lane).to(js, y)
		t = k.now()
		k.hide()

		for j := range 2 {
			i := b*2 + j
			k = d.tok("in", ins[i], x0[i], y, true).keep().until(t+float64(j)*0.5).enter(x0[i], gate, js, y, i < len(ins)-1).to(c, y)
			free = math.Max(free, k.now())
			k.until(free).to(h, y)
			arrive := k.now()
			k.hide()
			d.tok("out", ins[i], h, y, false).until(arrive+0.4).show().leave(h+65, slots[i], y, i > 0).keep()
			free = arrive + 0.6
		}
		t = free
	}

	return d
}

func headers() *diagram {
	const y, boxW = 70.0, 140.0

	d := newDiagram("headers",
		"The publisher reads X-Tenant-ID from the context header; NATS and HTTP carry it as X-Goflux-X-Tenant-ID; the subscriber strips the prefix and the handler reads X-Tenant-ID from msg.Header", 0, 120)
	in, wire := "X-Tenant-ID", "X-Goflux-X-Tenant-ID"
	start := 20 + tokenW(in)/2
	cx := []float64{220, 520, 760}
	labels := []string{"Publisher\nPublish(ctx, …)", "Transport\nNATS / HTTP", "Subscriber\nmsg.Header"}

	d.wire(start+tokenW(in)/2+10, y, cx[0]-boxW/2, y)
	for i, x := range cx {
		d.box(x, y, boxW, 50, labels[i])
		if i > 0 {
			d.wire(cx[i-1]+boxW/2, y, x-boxW/2, y)
		}
	}
	d.wire(cx[2]+boxW/2, y, cx[2]+boxW/2+20, y)
	d.text(20, y-45, "s", "goflux.WithHeader(ctx, …)")
	d.text((cx[0]+cx[1])/2, y-45, "", "wire header")
	d.text(cx[2]+boxW/2+20, y-45, "s", "msg.Header.Get(…)")
	end := cx[2] + boxW/2 + 30 + tokenW(in)/2
	d.w = end + tokenW(in)/2 + 40

	k := d.tok("in", in, start, y, true).keep().until(0.4).to(cx[0], y)
	done := k.now()
	k.hide()
	k = d.tok("mid", wire, cx[0], y, false).until(done).show().to(cx[1], y).wait(0.3).to(cx[2], y)
	done = k.now()
	k.hide()
	d.tok("out", in, cx[2], y, false).until(done).show().to(end, y).keep()

	return d
}

func requestReply() *diagram {
	const top, bottom, boxW = 45.0, 105.0, 140.0

	d := newDiagram("request-reply",
		"Request sends req through NATS to the responder's handler; the reply resp travels back and Request returns it", 720, 150)
	cx := []float64{170, 400, 630}
	labels := []string{"Requester\nRequest()", "NATS\norders.create", "Responder\nServe(handler)"}

	for i, x := range cx {
		d.box(x, (top+bottom)/2, boxW, 100, labels[i])
	}
	for _, y := range []float64{top, bottom} {
		d.wire(70, y, cx[0]-boxW/2, y)
		d.wire(cx[0]+boxW/2, y, cx[1]-boxW/2, y)
		d.wire(cx[1]+boxW/2, y, cx[2]-boxW/2, y)
	}

	k := d.tok("in", "req", 45, top, true).keep().until(0.4).to(cx[0], top).to(cx[1], top).to(cx[2], top)
	done := k.now()
	k.hide()
	d.tok("out", "resp", cx[2], bottom, false).until(done+0.4).show().to(cx[1], bottom).to(cx[0], bottom).to(45, bottom).keep()

	return d
}

func queueGroups() *diagram {
	const mid, boxW = 125.0, 150.0

	d := newDiagram("queue-groups",
		"Workers A and B share the queue group workers and split t1 to t4; Audit has no queue group and gets every message", 0, 250)
	ins := []string{"t1", "t2", "t3", "t4"}
	x0, gate := d.queue(ins, 20)
	nats := gate + 30 + 65
	split := nats + 65 + 25
	sub := split + 30 + boxW/2
	rows := []float64{45, 125, 205}

	d.wire(gate, mid, nats-65, mid)
	d.box(nats, mid, 130, 60, "NATS\ntasks.process")
	d.text(20, mid-40, "s", "Publish")

	names := []string{"Worker A\nqueue: workers", "Worker B\nqueue: workers", "Audit\nno queue group"}
	slots := make([][]float64, len(rows))
	for i, y := range rows {
		d.wire(nats+65, mid, split, mid, split, y, sub-boxW/2, y)
		d.wire(sub+boxW/2, y, sub+boxW/2+25, y)
		d.box(sub, y, boxW, 50, names[i])
		slots[i] = d.slots(ins, sub+boxW/2+35)
	}
	d.w = slots[2][3] + 40

	got := make([]int, len(rows))
	deliver := func(row int, text string, t float64) {
		y := rows[row]
		k := d.tok("in", text, nats, mid, false).until(t).show().to(split, mid).to(split, y).to(sub, y)
		done := k.now()
		k.hide()
		n := got[row]
		got[row]++
		d.tok("out", text, sub, y, false).until(done).show().leave(sub+boxW/2, slots[row][n], y, n > 0).keep()
	}

	for i, m := range ins {
		k := d.tok("in", m, x0[i], mid, true).keep().until(0.4+float64(i)*1.4).enter(x0[i], gate, nats, mid, i < len(ins)-1)
		done := k.now()
		k.hide()
		deliver(i%2, m, done)
		deliver(2, m, done)
	}

	return d
}

func middlewareChain() *diagram {
	const top, bottom, boxW = 45.0, 105.0, 150.0

	d := newDiagram("middleware-chain",
		"Chain(AutoAck, InjectMessageID)(handler): m1 passes AutoAck, then InjectMessageID, then the handler; nil returns through both and AutoAck acks", 0, 150)
	cx := []float64{150, 390, 630}
	labels := []string{"AutoAck\nack on nil", "InjectMessageID\nID into ctx", "handler\nreturns nil"}

	for i, x := range cx {
		d.box(x, (top+bottom)/2, boxW, 100, labels[i])
	}
	for i := 1; i < len(cx); i++ {
		d.wire(cx[i-1]+boxW/2, top, cx[i]-boxW/2, top)
		d.wire(cx[i-1]+boxW/2, bottom, cx[i]-boxW/2, bottom)
	}
	d.wire(55, top, cx[0]-boxW/2, top)
	d.wire(55, bottom, cx[0]-boxW/2, bottom)
	d.w = 720

	k := d.tok("in", "m1", 35, top, true).keep().until(0.4).to(cx[0], top).wait(0.3).to(cx[1], top).wait(0.3).to(cx[2], top)
	done := k.now()
	k.hide()
	k = d.tok("out", "nil", cx[2], bottom, false).until(done+0.4).show().to(cx[1], bottom).wait(0.3).to(cx[0], bottom)
	done = k.now()
	k.hide()
	d.tok("out", "ack", cx[0], bottom, false).until(done+0.3).show().to(35, bottom).keep()

	return d
}

func pipeNew() *diagram {
	return operator("pipe-new",
		"pipe.New with a filter for even values forwards 2 and 4 to the publisher; 1 and 3 are filtered and acked",
		"pipe.New(pub)\nfilter: even",
		[]string{"1", "2", "3", "4"}, [][]string{nil, {"2"}, nil, {"4"}}, "out")
}

func pipeMap() *diagram {
	return operator("pipe-map",
		"pipe.NewMap with strconv.Atoi publishes 1 and 3; x fails to map and the error returns to the transport",
		"pipe.NewMap(pub, fn)\nstrconv.Atoi",
		[]string{`"1"`, `"x"`, `"3"`}, [][]string{{"1"}, {"!x"}, {"3"}}, "mid")
}

func pipeFlatMap() *diagram {
	return operator("pipe-flatmap",
		"pipe.NewFlatMap expands order a into line items a1, a2 and order b into b1, publishing each one",
		"pipe.NewFlatMap(pub, fn)\nline items",
		[]string{"a", "b"}, [][]string{{"a1", "a2"}, {"b1"}}, "mid")
}

// operator feeds ins into one box. emit[i] lists the outputs for ins[i]; an
// output starting with "!" is an error.
func operator(name, alt, label string, ins []string, emit [][]string, class string) *diagram {
	const y, boxW = 55.0, 200.0

	d := newDiagram(name, alt, 0, 110)
	x0, gate := d.queue(ins, 20)
	cx := gate + 30 + boxW/2
	exit := cx + boxW/2

	var outs []string
	for _, e := range emit {
		for _, o := range e {
			outs = append(outs, strings.TrimPrefix(o, "!"))
		}
	}

	d.wire(gate, y, cx-boxW/2, y)
	d.wire(exit, y, exit+30, y)
	d.box(cx, y, boxW, 50, label)
	slots := d.slots(outs, exit+40)
	d.w = slots[len(slots)-1] + 40

	n := 0
	for i, m := range ins {
		k := d.tok("in", m, x0[i], y, true).keep().until(0.4+float64(i)*1.6).enter(x0[i], gate, cx, y, i < len(ins)-1)
		done := k.now()
		k.hide()

		for j, o := range emit[i] {
			c := class
			if strings.HasPrefix(o, "!") {
				c, o = "err", o[1:]
			}

			d.tok(c, o, cx, y, false).until(done+float64(j)*0.7).show().leave(exit, slots[n], y, n > 0).keep()
			n++
		}
	}

	return d
}

func retryPublisher() *diagram {
	const y, lane = 55.0, 108.0

	d := newDiagram("retry-publisher",
		"RetryPublisher with 3 attempts: the first two publish calls fail, it waits for the backoff and the third call succeeds", 0, 160)
	x0, gate := d.queue([]string{"m1"}, 20)
	r, p := gate+30+80, gate+30+80+220

	d.wire(gate, y, r-80, y)
	d.wire(r+80, y, p-70, y)
	d.wire(p+70, y, p+100, y)
	d.wire(p, y+25, p, lane, r, lane, r, y+25)
	d.text((r+p)/2, lane+30, "", "error: wait backoff(attempt), retry")
	d.box(r, y, 160, 50, "RetryPublisher\nmaxAttempts: 3")
	d.box(p, y, 140, 50, "Publisher")
	slots := d.slots([]string{"m1"}, p+110)
	d.w = slots[0] + 40

	k := d.tok("in", "m1", x0[0], y, true).keep().until(0.4).enter(x0[0], gate, r, y, false)
	t := k.now()
	k.hide()

	for attempt := range 3 {
		k = d.tok("in", "m1", r, y, false).until(t).show().to(p, y)
		t = k.now()
		k.hide()

		if attempt == 2 {
			d.tok("out", "m1", p, y, false).until(t).show().leave(p+70, slots[0], y, false).keep()

			break
		}

		k = d.tok("err", "err", p, y, false).until(t).show().to(p, lane).to(r, lane).to(r, y)
		t = k.now() + 0.6*float64(attempt+1)
		k.hide()
	}

	return d
}

func goflowIntegration() *diagram {
	const mid, boxW, step = 85.0, 110.0, 140.0

	d := newDiagram("goflow-integration",
		"Webhook events p5, p1, p4 go through ToStream, Filter drops p1 with priority below 3, Peek logs, Tee(2) copies p5 and p4 to two FromStream publishers for archive and notify", 0, 175)
	ins := []string{"p5", "p1", "p4"}
	x0, gate := d.queue(ins, 20)
	d.text(20, mid-40, "s", "HTTP POST /webhooks")

	labels := []string{"ToStream", "Filter\npriority ≥ 3", "Peek\nlog", "Tee(2)"}
	cx := make([]float64, len(labels))
	for i, l := range labels {
		cx[i] = gate + 30 + boxW/2 + float64(i)*step
		d.box(cx[i], mid, boxW, 50, l)
		if i > 0 {
			d.wire(cx[i-1]+boxW/2, mid, cx[i]-boxW/2, mid)
		}
	}
	d.wire(gate, mid, cx[0]-boxW/2, mid)

	tee := cx[3]
	split := tee + boxW/2 + 25
	pub := split + 30 + 65
	rows := []float64{40, 130}
	slots := make([][]float64, len(rows))
	for i, y := range rows {
		d.wire(tee+boxW/2, mid, split, mid, split, y, pub-65, y)
		d.wire(pub+65, y, pub+90, y)
		d.box(pub, y, 130, 50, []string{"FromStream\narchive", "FromStream\nnotify"}[i])
		slots[i] = d.slots([]string{"p5", "p4"}, pub+100)
	}
	d.w = slots[0][1] + 40

	n := 0
	for i, m := range ins {
		k := d.tok("in", m, x0[i], mid, true).keep().until(0.4+float64(i)*1.6).enter(x0[i], gate, cx[0], mid, i < len(ins)-1).to(cx[1], mid)
		if m == "p1" {
			k.hide()
			continue
		}

		k.to(cx[2], mid).to(cx[3], mid)
		done := k.now()
		k.hide()

		for row, y := range rows {
			c := d.tok("in", m, tee, mid, false).until(done).show().to(split, mid).to(split, y).to(pub, y)
			arrive := c.now()
			c.hide()
			d.tok("out", m, pub, y, false).until(arrive).show().leave(pub+65, slots[row][n], y, n > 0).keep()
		}
		n++
	}

	return d
}

// fan feeds ins through the boxes in series, then sends a copy of ins[i] to
// every row in route(i), one box per row. class returns the output class for
// ins[i].
func fan(name, alt, label string, boxes, rows, ins []string, route func(int) []int, class func(int) string, stagger float64) *diagram {
	const boxW, step, rowH = 110.0, 140.0, 85.0

	ys := make([]float64, len(rows))
	for r := range rows {
		ys[r] = 40 + float64(r)*rowH
	}
	mid := (ys[0] + ys[len(ys)-1]) / 2

	d := newDiagram(name, alt, 0, ys[len(ys)-1]+45)
	x0, gate := d.queue(ins, 20)
	d.text(20, mid-40, "s", label)

	cx := make([]float64, len(boxes))
	for i, l := range boxes {
		cx[i] = gate + 30 + boxW/2 + float64(i)*step
		d.box(cx[i], mid, boxW, 50, l)
		if i > 0 {
			d.wire(cx[i-1]+boxW/2, mid, cx[i]-boxW/2, mid)
		}
	}
	d.wire(gate, mid, cx[0]-boxW/2, mid)

	last := cx[len(cx)-1]
	split := last + boxW/2 + 25
	sub := split + 30 + 65
	routed := make([][]string, len(rows))
	for i, m := range ins {
		for _, r := range route(i) {
			routed[r] = append(routed[r], m)
		}
	}
	slots := make([][]float64, len(rows))
	for r, y := range ys {
		d.wire(last+boxW/2, mid, split, mid, split, y, sub-65, y)
		d.wire(sub+65, y, sub+90, y)
		d.box(sub, y, 130, 50, rows[r])
		slots[r] = d.slots(routed[r], sub+100)
		d.w = math.Max(d.w, slots[r][len(slots[r])-1]+40)
	}

	got := make([]int, len(rows))
	for i, m := range ins {
		k := d.tok("in", m, x0[i], mid, true).keep().until(0.4+float64(i)*stagger).enter(x0[i], gate, cx[0], mid, i < len(ins)-1)
		for _, x := range cx[1:] {
			k.to(x, mid)
		}
		done := k.now()
		k.hide()

		for _, r := range route(i) {
			y := ys[r]
			c := d.tok("in", m, last, mid, false).until(done).show().to(split, mid).to(split, y).to(sub, y)
			arrive := c.now()
			c.hide()
			n := got[r]
			got[r]++
			d.tok(class(i), m, sub, y, false).until(arrive+0.3).show().leave(sub+65, slots[r][n], y, n > 0).keep()
		}
	}

	return d
}

func goflowFanOut() *diagram {
	return fan("goflow-fanout",
		"FanOut(2) sends p5 and p7 to the archive publisher and p4 and p3 to the notify publisher; each message goes to one publisher only",
		"HTTP POST /webhooks", []string{"ToStream", "FanOut(2)"}, []string{"FromStream\narchive", "FromStream\nnotify"},
		[]string{"p5", "p4", "p7", "p3"}, func(i int) []int { return []int{i % 2} }, func(int) string { return "out" }, 1.4)
}

func streamTee() *diagram {
	return fan("stream-tee",
		"Tee(3) copies e1 and e2 to all three streams: archive, index and notify each get every message",
		"events.>", []string{"ToStream", "Tee(3)"}, []string{"ForEach\narchive", "ForEach\nindex", "ForEach\nnotify"},
		[]string{"e1", "e2"}, func(int) []int { return []int{0, 1, 2} }, func(int) string { return "out" }, 1.6)
}

func streamRoundRobin() *diagram {
	return fan("stream-round-robin",
		"FanOut(3) deals j1 to j4 round-robin: worker 0 gets j1 and j4, worker 1 gets j2, worker 2 gets j3",
		"jobs.>", []string{"ToStream", "FanOut(3)"}, []string{"ForEach\nworker 0", "ForEach\nworker 1", "ForEach\nworker 2"},
		[]string{"j1", "j2", "j3", "j4"}, func(i int) []int { return []int{i % 3} }, func(int) string { return "out" }, 1.4)
}

func goflowDistinct() *diagram {
	return operator("goflow-distinct",
		"Distinct by event ID passes e1, e2, e3 and drops the second e1 before Tee",
		"Distinct(ID)\nbefore Tee(2)",
		[]string{"e1", "e2", "e1", "e3"}, [][]string{{"e1"}, {"e2"}, nil, {"e3"}}, "out")
}

func goflowProcess() *diagram {
	return fan("goflow-process",
		"Process(8, fn) runs messages on up to 8 workers at once; fn acks t1, t2 and t4 and naks t3 for redelivery",
		"JetStream tasks.>", []string{"ToStream"}, []string{"worker 1\nof up to 8", "worker 2\nof up to 8"},
		[]string{"t1", "t2", "t3", "t4"}, func(i int) []int { return []int{i % 2} },
		func(i int) string {
			if i == 2 {
				return "err"
			}

			return "out"
		}, 0.5)
}

func goflowFanIn() *diagram {
	return fanIn("goflow-fanin",
		"ToStream turns the HTTP subscriber and the NATS subscriber into two streams; FanIn merges h1, n1, h2, n2 into one stream in arrival order",
		[2]string{"HTTP events", "NATS events.>"}, [2][]string{{"h1", "h2"}, {"n1", "n2"}})
}

func streamFanIn() *diagram {
	return fanIn("stream-fanin",
		"ToStream turns subscribers A and B into two streams; FanIn merges a1, b1, a2, b2 into one stream in arrival order",
		[2]string{"subA metrics.>", "subB metrics.>"}, [2][]string{{"a1", "a2"}, {"b1", "b2"}})
}

// fanIn merges two subscriber rows into one stream. The messages of the
// second row arrive in between those of the first.
func fanIn(name, alt string, labels [2]string, ins [2][]string) *diagram {
	const boxW, mid = 110.0, 85.0

	d := newDiagram(name, alt, 0, 175)
	rows := []struct {
		y     float64
		label string
		ins   []string
	}{
		{40, labels[0], ins[0]},
		{130, labels[1], ins[1]},
	}
	_, gate := d.queue(rows[0].ins, 20)
	ts := gate + 30 + boxW/2
	merge := ts + boxW/2 + 25
	fanIn := merge + 30 + boxW/2
	exit := fanIn + boxW/2

	var order []string
	for i := range ins[0] {
		order = append(order, ins[0][i], ins[1][i])
	}
	slots := d.slots(order, exit+40)
	d.w = slots[len(slots)-1] + 40

	d.box(fanIn, mid, boxW, 50, "FanIn")
	d.wire(exit, mid, exit+30, mid)

	type hit struct {
		t    float64
		text string
	}
	var hits []hit
	for r, row := range rows {
		x0, gate := d.queue(row.ins, 20)
		d.text(20, row.y-28, "s", row.label)
		d.wire(gate, row.y, ts-boxW/2, row.y)
		d.wire(ts+boxW/2, row.y, merge, row.y, merge, mid, fanIn-boxW/2, mid)
		d.box(ts, row.y, boxW, 44, "ToStream")
		for i, m := range row.ins {
			start := 0.4 + float64(i)*2.4 + float64(r)*1.2
			k := d.tok("in", m, x0[i], row.y, true).keep().until(start).enter(x0[i], gate, ts, row.y, i < len(row.ins)-1).to(merge, row.y).to(merge, mid).to(fanIn, mid)
			hits = append(hits, hit{k.now(), m})
			k.hide()
		}
	}
	// Outputs leave FanIn in arrival order.
	for len(hits) > 0 {
		j := 0
		for i, h := range hits {
			if h.t < hits[j].t {
				j = i
			}
		}
		h := hits[j]
		hits = append(hits[:j], hits[j+1:]...)
		n := len(order) - 1 - len(hits)
		d.tok("out", h.text, fanIn, mid, false).until(h.t).show().leave(exit, slots[n], mid, n > 0).keep()
	}

	return d
}

func httpIngest() *diagram {
	const boxW, mid = 120.0, 85.0

	d := newDiagram("http-ingest",
		"POST /orders sends o1, o2 and POST /payments sends p1, p2; each HTTP handler converts them into events and publishes them to NATS events.>; the store queue group splits the events between Store A and Store B, each collects a batch of 2 and inserts it into the database in one transaction", 0, 190)
	rows := []struct {
		y     float64
		label string
		ins   []string
	}{
		{40, "HTTP\n/orders", []string{"o1", "o2"}},
		{130, "HTTP\n/payments", []string{"p1", "p2"}},
	}
	_, gate := d.queue(rows[0].ins, 20)
	hb := gate + 30 + boxW/2
	merge := hb + boxW/2 + 25
	nats := merge + 30 + 65
	split := nats + 65 + 25
	st := split + 30 + boxW/2
	join := st + boxW/2 + 25
	db := join + 30 + 65
	slots := d.slots([]string{"[o1 p1]", "[o2 p2]"}, db+65+40)
	d.w = slots[1] + 40

	d.box(nats, mid, 130, 50, "NATS\nevents.>")
	d.box(db, mid, 130, 50, "Database\nINSERT batch")
	d.wire(db+65, mid, db+65+30, mid)
	d.text(st, 175, "", "queue group: event-store")

	type hit struct {
		t    float64
		text string
	}
	var hits []hit
	for r, row := range rows {
		x0, gate := d.queue(row.ins, 20)
		d.wire(gate, row.y, hb-boxW/2, row.y)
		d.wire(hb+boxW/2, row.y, merge, row.y, merge, mid, nats-65, mid)
		d.box(hb, row.y, boxW, 50, row.label)
		for i, m := range row.ins {
			start := 0.4 + float64(i)*2.4 + float64(r)*1.2
			k := d.tok("in", m, x0[i], row.y, true).keep().until(start).enter(x0[i], gate, hb, row.y, i < len(row.ins)-1)
			done := k.now()
			k.hide()
			k = d.tok("mid", m, hb, row.y, false).until(done).show().to(merge, row.y).to(merge, mid).to(nats, mid)
			hits = append(hits, hit{k.now(), m})
			k.hide()
		}
	}

	// The queue group delivers each event to one store instance.
	stores := []struct {
		y     float64
		label string
	}{{40, "Store A\nBatch(2)"}, {130, "Store B\nBatch(2)"}}
	for _, s := range stores {
		d.wire(nats+65, mid, split, mid, split, s.y, st-boxW/2, s.y)
		d.wire(st+boxW/2, s.y, join, s.y, join, mid, db-65, mid)
		d.box(st, s.y, boxW, 50, s.label)
	}

	got := make([][]hit, len(stores))
	for n := range hits {
		j := n
		for i := n; i < len(hits); i++ {
			if hits[i].t < hits[j].t {
				j = i
			}
		}
		hits[n], hits[j] = hits[j], hits[n]
		h := hits[n]
		s := n / 2
		y := stores[s].y
		k := d.tok("mid", h.text, nats, mid, false).until(h.t).show().to(split, mid).to(split, y).to(st, y)
		got[s] = append(got[s], hit{k.now(), h.text})
		k.hide()
	}

	for s, g := range got {
		y := stores[s].y
		text := "[" + g[0].text + " " + g[1].text + "]"
		k := d.tok("mid", text, st, y, false).until(g[1].t+0.3).show().to(join, y).to(join, mid).to(db, mid)
		done := k.now()
		k.hide()
		d.tok("out", text, db, mid, false).until(done+0.3).show().leave(db+65, slots[s], mid, s > 0).keep()
	}

	return d
}

// ~ Scene

type diagram struct {
	name, alt   string
	w, h        float64
	back, front strings.Builder
	tokens      []*token
}

func newDiagram(name, alt string, w, h float64) *diagram {
	return &diagram{name: name, alt: alt, w: w, h: h}
}

// queue lays out waiting tokens from x and returns their centers and the
// x where they leave the queue.
func (d *diagram) queue(ins []string, x float64) ([]float64, float64) {
	xs := d.slots(ins, x)
	last := len(ins) - 1

	return xs, xs[last] + tokenW(ins[last])/2 + 12
}

// slots returns the centers of tokens placed side by side from x.
func (d *diagram) slots(texts []string, x float64) []float64 {
	xs := make([]float64, len(texts))
	for i, t := range texts {
		w := tokenW(t)
		xs[i] = x + w/2
		x += w + gap
	}

	return xs
}

func (d *diagram) wire(pts ...float64) {
	var b strings.Builder
	for i := 0; i < len(pts); i += 2 {
		cmd := "L"
		if i == 0 {
			cmd = "M"
		}
		fmt.Fprintf(&b, "%s%.1f %.1f", cmd, pts[i], pts[i+1])
	}
	fmt.Fprintf(&d.back, `<path class="wire" d="%s"/>`, b.String())
}

// box draws a box on top of the tokens. A second label line is drawn smaller.
func (d *diagram) box(cx, cy, w, h float64, label string) {
	fmt.Fprintf(&d.front, `<rect class="box" x="%.1f" y="%.1f" width="%.1f" height="%.1f" rx="10"/>`, cx-w/2, cy-h/2, w, h)
	title, sub, ok := strings.Cut(label, "\n")
	if !ok {
		fmt.Fprintf(&d.front, `<text class="op" x="%.1f" y="%.1f">%s</text>`, cx, cy, html.EscapeString(title))
		return
	}
	fmt.Fprintf(&d.front, `<text class="op" x="%.1f" y="%.1f">%s</text>`, cx, cy-9, html.EscapeString(title))
	fmt.Fprintf(&d.front, `<text class="sub" x="%.1f" y="%.1f">%s</text>`, cx, cy+10, html.EscapeString(sub))
}

// zone draws a dashed process boundary behind the tokens.
func (d *diagram) zone(x, y, w, h float64) {
	fmt.Fprintf(&d.back, `<rect class="zone" x="%.1f" y="%.1f" width="%.1f" height="%.1f" rx="12"/>`, x, y, w, h)
}

// text draws a label. anchor is "s" (start), "e" (end) or "" (middle).
func (d *diagram) text(x, y float64, anchor, s string) {
	fmt.Fprintf(&d.back, `<text class="lbl %s" x="%.1f" y="%.1f">%s</text>`, anchor, x, y, html.EscapeString(s))
}

func (d *diagram) tok(class, text string, x, y float64, visible bool) *token {
	o := 0.0
	if visible {
		o = 1
	}
	k := &token{class: class, text: text, steps: []step{{0, x, y, o}}}
	d.tokens = append(d.tokens, k)

	return k
}

// render returns the animated SVG, or a static frame if at >= 0.
func (d *diagram) render(at float64) string {
	end := 0.0
	for _, k := range d.tokens {
		end = math.Max(end, k.now())
	}
	end += hold
	dur := end + 0.6

	var b strings.Builder
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %.0f %.0f" width="%.0f" height="%.0f" role="img" aria-label="%s">`+"\n",
		d.w, d.h, d.w, d.h, html.EscapeString(d.alt))
	b.WriteString(style)

	if at < 0 {
		b.WriteString("<style>\n")
		for i, k := range d.tokens {
			r := k.steps[k.rest]
			fmt.Fprintf(&b, "@keyframes t%d{", i)
			for _, s := range k.loop(end, dur) {
				fmt.Fprintf(&b, "%.3f%%{transform:translate(%.1fpx,%.1fpx);opacity:%g}", s.t/dur*100, s.x-r.x, s.y-r.y, s.o)
			}
			fmt.Fprintf(&b, "}.t%d{animation:t%d %.2fs linear infinite}\n", i, i, dur)
		}
		b.WriteString("</style>\n")
	}

	b.WriteString(d.back.String())
	for i, k := range d.tokens {
		r := k.steps[k.rest]
		vis := "h"
		if r.o == 1 {
			vis = "k"
		}
		attr := ""
		if at >= 0 {
			s := k.frame(at, end, dur)
			attr = fmt.Sprintf(` transform="translate(%.1f %.1f)" opacity="%g"`, s.x-r.x, s.y-r.y, s.o)
		}
		w := tokenW(k.text)
		fmt.Fprintf(&b, `<g class="t%d %s"%s><rect class="%s" x="%.1f" y="%.1f" width="%.1f" height="%.0f" rx="%.0f"/><text x="%.1f" y="%.1f">%s</text></g>`,
			i, vis, attr, k.class, r.x-w/2, r.y-tokenH/2, w, tokenH, tokenH/2, r.x, r.y, html.EscapeString(k.text))
	}
	b.WriteString(d.front.String())
	b.WriteString("\n</svg>\n")

	return b.String()
}

func tokenW(text string) float64 {
	return math.Max(36, 8.4*float64(utf8.RuneCountInString(text))+20)
}

// ~ Tokens

type step struct{ t, x, y, o float64 }

// token is a pill that moves through the scene. rest is the step it is drawn
// at without animation.
type token struct {
	class, text string
	steps       []step
	rest        int
}

func (k *token) last() step   { return k.steps[len(k.steps)-1] }
func (k *token) now() float64 { return k.last().t }

func (k *token) add(s step) *token {
	l := k.last()
	if s == l {
		return k
	}
	if s.t <= l.t {
		s.t = l.t + 0.01
	}
	k.steps = append(k.steps, s)

	return k
}

// keep marks the current step as the position without animation.
func (k *token) keep() *token { k.rest = len(k.steps) - 1; return k }

func (k *token) wait(dt float64) *token { s := k.last(); s.t += dt; return k.add(s) }

func (k *token) until(t float64) *token {
	if s := k.last(); t > s.t {
		s.t = t
		k.add(s)
	}

	return k
}

func (k *token) show() *token { s := k.last(); s.t += fade; s.o = 1; return k.add(s) }
func (k *token) hide() *token { s := k.last(); s.t += fade; s.o = 0; return k.add(s) }

func (k *token) to(x, y float64) *token {
	s := k.last()
	s.t += math.Hypot(x-s.x, y-s.y) / speed
	s.x, s.y = x, y

	return k.add(s)
}

// enter moves a waiting token into the box at boxX, hopping over the tokens
// still waiting to its right.
func (k *token) enter(slotX, gateX, boxX, y float64, hop bool) *token {
	if hop {
		k.to(slotX, y-hopH).to(gateX-20, y-hopH)
	}

	return k.to(gateX, y).to(boxX, y)
}

// leave moves a token out of a box to its slot, hopping over the tokens that
// already arrived.
func (k *token) leave(gateX, slotX, y float64, hop bool) *token {
	k.to(gateX, y)
	if hop {
		k.to(gateX+15, y-hopH).to(slotX, y-hopH)
	}

	return k.to(slotX, y)
}

// loop appends the fade out at end and the reset to the first step.
func (k *token) loop(end, dur float64) []step {
	l, f := k.last(), k.steps[0]

	return append(append([]step(nil), k.steps...),
		step{end, l.x, l.y, l.o},
		step{end + 0.3, l.x, l.y, 0},
		step{end + 0.31, f.x, f.y, 0},
		step{dur, f.x, f.y, f.o},
	)
}

// frame interpolates the token state at t.
func (k *token) frame(t, end, dur float64) step {
	ss := k.loop(end, dur)
	for i := 1; i < len(ss); i++ {
		a, b := ss[i-1], ss[i]
		if t > b.t {
			continue
		}
		p := 0.0
		if b.t > a.t {
			p = (t - a.t) / (b.t - a.t)
		}

		return step{t, a.x + (b.x-a.x)*p, a.y + (b.y-a.y)*p, a.o + (b.o-a.o)*p}
	}

	return ss[len(ss)-1]
}
