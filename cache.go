package proton_api_bridge

import (
	"context"
	"log"
	"sync"

	"github.com/ProtonMail/gopenpgp/v2/crypto"
	"github.com/henrybear327/go-proton-api"
)

type cacheEntry struct {
	link *proton.Link
	kr   *crypto.KeyRing
}
type cache struct {
	data           map[string]*cacheEntry
	disableCaching bool

	sync.RWMutex
}

func newLinkCache(disableCaching bool) *cache {
	return &cache{
		data:           make(map[string]*cacheEntry),
		disableCaching: disableCaching,
	}
}

func (cache *cache) _get(linkID string) *cacheEntry {
	if cache.disableCaching {
		return nil
	}

	cache.RLock()
	defer cache.RUnlock()

	if val, ok := cache.data[linkID]; ok {
		return val
	}
	return nil
}

func (cache *cache) _insert(link *proton.Link, kr *crypto.KeyRing) {
	if cache.disableCaching {
		return
	}

	cache.Lock()
	defer cache.Unlock()

	cache.data[link.LinkID] = &cacheEntry{
		link: link,
		kr:   kr,
	}
}

func (protonDrive *ProtonDrive) getLink(ctx context.Context, linkID string) (*proton.Link, error) {
	// attempt to get from cache first
	if data := protonDrive.cache._get(linkID); data != nil && data.link != nil {
		// log.Println("From cache (Link)")
		return data.link, nil
	}

	// log.Println("Not from cache (Link)")
	// no cached data, fetch
	link, err := protonDrive.c.GetLink(ctx, protonDrive.MainShare.ShareID, linkID)
	if err != nil {
		return nil, err
	}

	// populate cache
	protonDrive.cache._insert(&link, nil)

	return &link, nil
}

func (protonDrive *ProtonDrive) _fetchNodeKR(ctx context.Context, link *proton.Link) (*crypto.KeyRing, error) {
	if link.ParentLinkID == "" {
		if data := protonDrive.cache._get(link.LinkID); data != nil && data.kr != nil {
			return data.kr, nil
		}
		nodeKR, err := link.GetKeyRing(protonDrive.MainShareKR, protonDrive.AddrKR)
		if err != nil {
			return nil, err
		}
		protonDrive.cache._insert(link, nodeKR)

		return nodeKR, nil
	}

	parentLink, err := protonDrive.getLink(ctx, link.ParentLinkID)
	if err != nil {
		return nil, err
	}

	// parentNodeKR is used to decrypt the current node's KR, as each node has its keyring, which can be decrypted by its parent
	parentNodeKR, err := protonDrive.getNodeKR(ctx, parentLink)
	if err != nil {
		return nil, err
	}

	if data := protonDrive.cache._get(link.LinkID); data != nil && data.kr != nil {
		return data.kr, nil
	}
	nodeKR, err := link.GetKeyRing(parentNodeKR, protonDrive.AddrKR)
	if err != nil {
		return nil, err
	}
	protonDrive.cache._insert(link, nodeKR)

	return nodeKR, nil
}

func (protonDrive *ProtonDrive) getNodeKR(ctx context.Context, link *proton.Link) (*crypto.KeyRing, error) {
	// attempt to get from cache first
	if data := protonDrive.cache._get(link.LinkID); data != nil && data.kr != nil {
		log.Println("From cache (kr)")
		return data.kr, nil
	}

	log.Println("Not from cache (kr)")
	// no cached data, fetch
	kr, err := protonDrive._fetchNodeKR(ctx, link)
	if err != nil {
		return nil, err
	}

	// populate cache
	protonDrive.cache._insert(link, kr)

	return kr, nil
}
