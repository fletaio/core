package formulator

import (
	"bytes"
	crand "crypto/rand"
	"encoding/binary"
	"io"
	"log"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/fletaio/common"
	"github.com/fletaio/common/hash"
	"github.com/fletaio/common/util"
	"github.com/fletaio/core/key"
	"github.com/fletaio/core/message_def"
	"github.com/fletaio/framework/chain/mesh"
	"github.com/fletaio/framework/message"
)

// EventHandler is a event handler of the mesh
type EventHandler interface {
	OnObserverConnected(p *Peer)
	OnObserverDisconnected(p *Peer)
	OnRecv(p mesh.Peer, r io.Reader, t message.Type) error
}

// Mesh is a connection mesh of the formulator
type Mesh struct {
	sync.Mutex
	Key           key.Key
	Formulator    common.Address
	NetAddressMap map[common.PublicHash]string
	handler       mesh.EventHandler
	peerHash      map[string]*Peer
}

// NewMesh returns a Mesh
func NewMesh(Key key.Key, Formulator common.Address, NetAddressMap map[common.PublicHash]string, handler mesh.EventHandler) *Mesh {
	ms := &Mesh{
		Key:           Key,
		Formulator:    Formulator,
		NetAddressMap: NetAddressMap,
		handler:       handler,
		peerHash:      map[string]*Peer{},
	}
	return ms
}

// Add is not implemented and not used
func (ms *Mesh) Add(netAddr string, doForce bool) {
}

// Remove is not implemented and not used
func (ms *Mesh) Remove(netAddr string) {
}

// RemoveByID is not implemented and not used
func (ms *Mesh) RemoveByID(ID string) {
}

// Ban is not implemented and not used
func (ms *Mesh) Ban(netAddr string, Seconds uint32) {
}

// BanByID is not implemented and not used
func (ms *Mesh) BanByID(ID string, Seconds uint32) {
}

// Unban is not implemented and not used
func (ms *Mesh) Unban(netAddr string) {
}

// Peers returns peers of the mesh
func (ms *Mesh) Peers() []mesh.Peer {
	ms.Lock()
	defer ms.Unlock()

	peers := []mesh.Peer{}
	for _, p := range ms.peerHash {
		peers = append(peers, p)
	}
	return peers
}

// Run runs a mesh network
func (ms *Mesh) Run() error {
	var wg sync.WaitGroup
	ObPubHash := common.NewPublicHash(ms.Key.PublicKey())
	for PubHash, v := range ms.NetAddressMap {
		if !PubHash.Equal(ObPubHash) {
			wg.Add(1)
			go func(pubhash common.PublicHash, NetAddr string) {
				defer wg.Done()

				time.Sleep(1 * time.Second)
				for {
					ms.Lock()
					_, has := ms.peerHash[pubhash.String()]
					ms.Unlock()
					if !has {
						if err := ms.client(NetAddr, pubhash); err != nil {
							log.Println("[client]", err, NetAddr)
						}
					}
					time.Sleep(1 * time.Second)
				}
			}(PubHash, v)
		}
	}
	wg.Wait()
	return nil
}

// RemovePeer removes peers from the mesh
func (ms *Mesh) RemovePeer(p *Peer) {
	ms.Lock()
	delete(ms.peerHash, p.ID())
	ms.Unlock()

	p.conn.Close()
	ms.handler.OnDisconnected(p)
}

// SendTo sends a message to the target peer
func (ms *Mesh) SendTo(id string, m message.Message) error {
	ms.Lock()
	p, has := ms.peerHash[id]
	ms.Unlock()
	if !has {
		return ErrUnknownPeer
	}

	if err := p.Send(m); err != nil {
		ms.RemovePeer(p)
		return err
	}
	return nil
}

// BroadcastMessage sends a message to the peers
func (ms *Mesh) BroadcastMessage(m message.Message) error {
	var buffer bytes.Buffer
	if _, err := util.WriteUint64(&buffer, uint64(m.Type())); err != nil {
		return err
	}
	if _, err := m.WriteTo(&buffer); err != nil {
		return err
	}
	data := buffer.Bytes()

	peers := []*Peer{}
	ms.Lock()
	for _, p := range ms.peerHash {
		peers = append(peers, p)
	}
	ms.Unlock()
	for _, p := range peers {
		if err := p.SendRaw(data); err != nil {
			log.Println(err)
			ms.RemovePeer(p)
		}
	}
	return nil
}

func (ms *Mesh) client(Address string, TargetPubHash common.PublicHash) error {
	conn, err := net.DialTimeout("tcp", Address, 10*time.Second)
	if err != nil {
		return err
	}
	defer conn.Close()

	if err := ms.recvHandshake(conn); err != nil {
		log.Println("[recvHandshake]", err)
		return err
	}
	pubhash, err := ms.sendHandshake(conn)
	if err != nil {
		log.Println("[sendHandshake]", err)
		return err
	}
	if !pubhash.Equal(TargetPubHash) {
		return common.ErrInvalidPublicHash
	}
	if _, has := ms.NetAddressMap[pubhash]; !has {
		return ErrNotAllowedPublicHash
	}

	p := NewPeer(conn, pubhash)

	ms.Lock()
	old, has := ms.peerHash[p.ID()]
	ms.peerHash[p.ID()] = p
	ms.Unlock()
	if has {
		ms.RemovePeer(old)
	}
	defer ms.RemovePeer(p)

	if err := ms.handleConnection(p); err != nil {
		return err
	}
	return nil
}

func (ms *Mesh) handleConnection(p *Peer) error {
	log.Println(ms.Formulator.String(), "Connected To", p.ID())

	ms.handler.OnConnected(p)

	var pingCount uint64
	pingCountLimit := uint64(3)
	pingTimer := time.NewTimer(10 * time.Second)
	go func() {
		for {
			select {
			case <-pingTimer.C:
				if err := p.Send(&message_def.PingMessage{}); err != nil {
					p.conn.Close()
					return
				}
				if atomic.AddUint64(&pingCount, 1) > pingCountLimit {
					p.conn.Close()
					return
				}
			}
		}
	}()
	for {
		t, bs, err := p.ReadMessageData()
		if err != nil {
			return err
		}
		atomic.SwapUint64(&pingCount, 0)
		if bs == nil {
			// Because a Message is zero size, so do not need to consume the body
			continue
		}

		if err := ms.handler.OnRecv(p, bytes.NewReader(bs), t); err != nil {
			return err
		}
	}
}

func (ms *Mesh) recvHandshake(conn net.Conn) error {
	//log.Println("recvHandshake")
	req := make([]byte, 40)
	if _, err := util.FillBytes(conn, req); err != nil {
		return err
	}
	timestamp := binary.LittleEndian.Uint64(req[32:])
	diff := time.Duration(uint64(time.Now().UnixNano()) - timestamp)
	if diff < 0 {
		diff = -diff
	}
	if diff > time.Second*30 {
		return ErrInvalidTimestamp
	}
	//log.Println("sendHandshakeAck")
	h := hash.Hash(req)
	if sig, err := ms.Key.Sign(h); err != nil {
		return err
	} else if _, err := conn.Write(sig[:]); err != nil {
		return err
	}
	return nil
}

func (ms *Mesh) sendHandshake(conn net.Conn) (common.PublicHash, error) {
	//log.Println("sendHandshake")
	req := make([]byte, 60)
	if _, err := crand.Read(req[:32]); err != nil {
		return common.PublicHash{}, err
	}
	copy(req[32:], ms.Formulator[:])
	binary.LittleEndian.PutUint64(req[52:], uint64(time.Now().UnixNano()))
	if _, err := conn.Write(req); err != nil {
		return common.PublicHash{}, err
	}
	//log.Println("recvHandshakeAsk")
	h := hash.Hash(req)
	var sig common.Signature
	if _, err := sig.ReadFrom(conn); err != nil {
		return common.PublicHash{}, err
	}
	pubkey, err := common.RecoverPubkey(h, sig)
	if err != nil {
		return common.PublicHash{}, err
	}
	pubhash := common.NewPublicHash(pubkey)
	return pubhash, nil
}
