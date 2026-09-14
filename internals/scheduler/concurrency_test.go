package scheduler

import (
	"io"
	"net"
	"testing"
	"time"

	"github.com/seeniolabode/gotorrent/internals/peers"
	"github.com/seeniolabode/gotorrent/internals/types"
)

func TestPeerCapAndReplacement(t *testing.T) {
	s, _ := newTestScheduler(t)
	if err := s.prepareStorage(); err != nil {
		t.Fatal(err)
	}
	s.maxPeers = 2
	s.requestTimeout = time.Second
	accepted := make(chan int, 3)
	discovered := make([]types.Peer, 3)
	connections := make([]chan struct{}, 3)
	for i := range 3 {
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { listener.Close() })
		addr := listener.Addr().(*net.TCPAddr)
		discovered[i] = types.Peer{IP: addr.IP, Port: uint16(addr.Port)}
		connections[i] = make(chan struct{})
		go func() {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			defer conn.Close()
			conn.SetDeadline(time.Now().Add(2 * time.Second))
			request := make([]byte, 68)
			if _, err = io.ReadFull(conn, request); err != nil {
				return
			}
			if _, err = conn.Write(request); err != nil {
				return
			}
			accepted <- i
			<-connections[i]
		}()
	}
	result := make(chan error, 1)
	go func() {
		result <- s.runPeers(discovered, peers.HandshakeOptions{PeerID: types.PeerID{1}, InfoHash: s.torrent.Metadata.InfoHash})
	}()
	awaitPeer := func() int {
		t.Helper()
		select {
		case i := <-accepted:
			return i
		case <-time.After(2 * time.Second):
			t.Fatal("peer did not connect")
			return -1
		}
	}
	first, second := awaitPeer(), awaitPeer()
	select {
	case <-accepted:
		t.Fatal("exceeded peer cap")
	case <-time.After(20 * time.Millisecond):
	}
	close(connections[first])
	third := awaitPeer()
	close(connections[second])
	close(connections[third])
	select {
	case err := <-result:
		if err == nil {
			t.Fatal("incomplete download reported success")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("scheduler did not finish")
	}
}
