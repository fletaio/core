package observer

import (
	"bytes"
	crand "crypto/rand"
	"encoding/binary"
	"io"
	"log"
	"net"
	"runtime"
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

// ObserverMeshDeligator deligates unhandled messages of the observer mesh
type ObserverMeshDeligator interface {
	OnRecv(p mesh.Peer, r io.Reader, t message.Type) error
}

type ObserverMesh struct {
	sync.Mutex
	Key           key.Key
	NetAddressMap map[common.PublicHash]string
	clientPeerMap map[common.PublicHash]*Peer
	serverPeerMap map[common.PublicHash]*Peer
	deligator     ObserverMeshDeligator
	handler       mesh.EventHandler
}

func NewObserverMesh(Key key.Key, NetAddressMap map[common.PublicHash]string, Deligator ObserverMeshDeligator, handler mesh.EventHandler) *ObserverMesh {
	ms := &ObserverMesh{
		Key:           Key,
		NetAddressMap: NetAddressMap,
		clientPeerMap: map[common.PublicHash]*Peer{},
		serverPeerMap: map[common.PublicHash]*Peer{},
		deligator:     Deligator,
		handler:       handler,
	}
	return ms
}

func (ms *ObserverMesh) Add(netAddr string, doForce bool) {
	log.Println("ObserverMesh", "Add", netAddr, doForce)
}
func (ms *ObserverMesh) Remove(netAddr string) {
	log.Println("ObserverMesh", "Remove", netAddr)
}
func (ms *ObserverMesh) RemoveByID(ID string) {
	log.Println("ObserverMesh", "RemoveByID", ID)
}
func (ms *ObserverMesh) Ban(netAddr string, Seconds uint32) {
	log.Println("ObserverMesh", "Ban", netAddr, Seconds)
}
func (ms *ObserverMesh) BanByID(ID string, Seconds uint32) {
	log.Println("ObserverMesh", "BanByID", ID, Seconds)
}
func (ms *ObserverMesh) Unban(netAddr string) {
	log.Println("ObserverMesh", "Unban", netAddr)
}
func (ms *ObserverMesh) Peers() []mesh.Peer {
	peerMap := map[common.PublicHash]*Peer{}
	ms.Lock()
	for _, p := range ms.clientPeerMap {
		peerMap[p.pubhash] = p
	}
	for _, p := range ms.serverPeerMap {
		peerMap[p.pubhash] = p
	}
	ms.Unlock()

	peers := []mesh.Peer{}
	for _, p := range peerMap {
		peers = append(peers, p)
	}
	return peers
}

func (ms *ObserverMesh) Run(BindAddress string) {
	ObPubHash := common.NewPublicHash(ms.Key.PublicKey())
	for PubHash, v := range ms.NetAddressMap {
		if !PubHash.Equal(ObPubHash) {
			go func(pubhash common.PublicHash, NetAddr string) {
				time.Sleep(1 * time.Second)
				for {
					ms.Lock()
					_, hasC := ms.clientPeerMap[pubhash]
					_, hasS := ms.serverPeerMap[pubhash]
					ms.Unlock()
					if !hasC && !hasS {
						if err := ms.client(NetAddr, pubhash); err != nil {
							log.Println("[client]", err, NetAddr)
						}
					}
					time.Sleep(1 * time.Second)
				}
			}(PubHash, v)
		}
	}
	if err := ms.server(BindAddress); err != nil {
		panic(err)
	}
}

// RemovePeer removes peers from the mesh
func (ms *ObserverMesh) RemovePeer(p *Peer) {
	ms.Lock()
	pc, hasClient := ms.clientPeerMap[p.pubhash]
	if hasClient {
		delete(ms.clientPeerMap, p.pubhash)
	}
	ps, hasServer := ms.serverPeerMap[p.pubhash]
	if hasServer {
		delete(ms.serverPeerMap, p.pubhash)
	}
	ms.Unlock()

	if hasClient {
		pc.conn.Close()
		ms.handler.OnDisconnected(pc)
	}
	if hasServer {
		ps.conn.Close()
		ms.handler.OnDisconnected(ps)
	}
}

// RemovePeerInMap removes peers from the mesh in the map
func (ms *ObserverMesh) RemovePeerInMap(p *Peer, peerMap map[common.PublicHash]*Peer) {
	ms.Lock()
	delete(peerMap, p.pubhash)
	ms.Unlock()

	p.conn.Close()
	ms.handler.OnDisconnected(p)
}

// SendTo sends a message to the observer
func (ms *ObserverMesh) SendTo(PublicHash common.PublicHash, m message.Message) error {
	ms.Lock()
	var p *Peer
	if cp, has := ms.clientPeerMap[PublicHash]; has {
		p = cp
	} else if sp, has := ms.serverPeerMap[PublicHash]; has {
		p = sp
	}
	ms.Unlock()
	if p == nil {
		return ErrUnknownObserver
	}

	if err := p.Send(m); err != nil {
		log.Println(err)
		ms.RemovePeer(p)
	}
	return nil
}

// BroadcastRaw sends a message to all peers
func (ms *ObserverMesh) BroadcastRaw(bs []byte) error {
	peerMap := map[common.PublicHash]*Peer{}
	ms.Lock()
	for _, p := range ms.clientPeerMap {
		peerMap[p.pubhash] = p
	}
	for _, p := range ms.serverPeerMap {
		peerMap[p.pubhash] = p
	}
	ms.Unlock()

	for _, p := range peerMap {
		if err := p.SendRaw(bs); err != nil {
			log.Println(err)
			ms.RemovePeer(p)
		}
	}
	runtime.Gosched()
	return nil
}

// BroadcastMessage sends a message to all peers
func (ms *ObserverMesh) BroadcastMessage(m message.Message) error {
	var buffer bytes.Buffer
	if _, err := util.WriteUint64(&buffer, uint64(m.Type())); err != nil {
		return err
	}
	if _, err := m.WriteTo(&buffer); err != nil {
		return err
	}
	data := buffer.Bytes()

	peerMap := map[common.PublicHash]*Peer{}
	ms.Lock()
	for _, p := range ms.clientPeerMap {
		peerMap[p.pubhash] = p
	}
	for _, p := range ms.serverPeerMap {
		peerMap[p.pubhash] = p
	}
	ms.Unlock()

	for _, p := range peerMap {
		if err := p.SendRaw(data); err != nil {
			log.Println(err)
			ms.RemovePeer(p)
		}
	}
	runtime.Gosched()
	return nil
}

func (ms *ObserverMesh) client(Address string, TargetPubHash common.PublicHash) error {
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
	old, has := ms.clientPeerMap[pubhash]
	ms.clientPeerMap[pubhash] = p
	ms.Unlock()
	if has {
		ms.RemovePeerInMap(old, ms.clientPeerMap)
	}
	defer ms.RemovePeerInMap(p, ms.clientPeerMap)

	if err := ms.handleConnection(p); err != nil {
		log.Println("[handleConnection]", err)
	}
	return nil
}

func (ms *ObserverMesh) server(BindAddress string) error {
	lstn, err := net.Listen("tcp", BindAddress)
	if err != nil {
		return err
	}
	log.Println(common.NewPublicHash(ms.Key.PublicKey()), "Start to Listen", BindAddress)
	for {
		conn, err := lstn.Accept()
		if err != nil {
			return err
		}
		go func() {
			defer conn.Close()

			pubhash, err := ms.sendHandshake(conn)
			if err != nil {
				log.Println("[sendHandshake]", err)
				return
			}
			if _, has := ms.NetAddressMap[pubhash]; !has {
				log.Println("ErrInvalidPublicHash")
				return
			}
			if err := ms.recvHandshake(conn); err != nil {
				log.Println("[recvHandshakeAck]", err)
				return
			}

			p := NewPeer(conn, pubhash)
			ms.Lock()
			old, has := ms.serverPeerMap[pubhash]
			ms.serverPeerMap[pubhash] = p
			ms.Unlock()
			if has {
				ms.RemovePeerInMap(old, ms.serverPeerMap)
			}
			defer ms.RemovePeerInMap(p, ms.serverPeerMap)

			if err := ms.handleConnection(p); err != nil {
				log.Println("[handleConnection]", err)
			}
		}()
	}
}

func (ms *ObserverMesh) handleConnection(p *Peer) error {
	log.Println(common.NewPublicHash(ms.Key.PublicKey()).String(), "Connected", p.pubhash.String())

	ms.handler.OnConnected(p)

	var pingCount uint64
	pingCountLimit := uint64(3)
	pingTicker := time.NewTicker(10 * time.Second)
	go func() {
		for {
			select {
			case <-pingTicker.C:
				if err := p.Send(&message_def.PingMessage{}); err != nil {
					ms.RemovePeer(p)
					return
				}
				if atomic.AddUint64(&pingCount, 1) > pingCountLimit {
					ms.RemovePeer(p)
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

		if err := ms.deligator.OnRecv(p, bytes.NewReader(bs), t); err != nil {
			return err
		}
	}
}

func (ms *ObserverMesh) recvHandshake(conn net.Conn) error {
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

func (ms *ObserverMesh) sendHandshake(conn net.Conn) (common.PublicHash, error) {
	//log.Println("sendHandshake")
	req := make([]byte, 40)
	if _, err := crand.Read(req[:32]); err != nil {
		return common.PublicHash{}, err
	}
	binary.LittleEndian.PutUint64(req[32:], uint64(time.Now().UnixNano()))
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
