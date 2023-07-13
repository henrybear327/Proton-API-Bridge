package proton_api_bridge

import (
	"context"
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

func newCache(disableCaching bool) *cache {
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

	if data, ok := cache.data[linkID]; ok {
		return data
	}
	return nil
}

func (cache *cache) _insert(linkID string, link *proton.Link, kr *crypto.KeyRing) {
	if cache.disableCaching {
		return
	}

	cache.Lock()
	defer cache.Unlock()

	cache.data[linkID] = &cacheEntry{
		link: link,
		kr:   kr,
	}
}

/* The original non-caching version, which resolves the keyring recursively */
func (protonDrive *ProtonDrive) _getLinkKRByID(ctx context.Context, linkID string) (*crypto.KeyRing, error) {
	if linkID == "" {
		// most likely someone requested root link's parent link, which happen to be ""
		// return protonDrive.MainShareKR.Copy() // we need to return a deep copy since the keyring will be freed by the caller when it finishes using the keyring -> now we go through caching, so we won't clear kr

		return protonDrive.MainShareKR, nil
	}

	link, err := protonDrive.getLink(ctx, linkID)
	if err != nil {
		return nil, err
	}

	return protonDrive._getLinkKR(ctx, link)
}

/* The original non-caching version, which resolves the keyring recursively */
func (protonDrive *ProtonDrive) _getLinkKR(ctx context.Context, link *proton.Link) (*crypto.KeyRing, error) {
	if link.ParentLinkID == "" { // link is rootLink
		nodeKR, err := link.GetKeyRing(protonDrive.MainShareKR, protonDrive.AddrKR)
		if err != nil {
			return nil, err
		}

		return nodeKR, nil
	}

	parentLink, err := protonDrive.getLink(ctx, link.ParentLinkID)
	if err != nil {
		return nil, err
	}

	// parentNodeKR is used to decrypt the current node's KR, as each node has its keyring, which can be decrypted by its parent
	parentNodeKR, err := protonDrive._getLinkKR(ctx, parentLink)
	if err != nil {
		return nil, err
	}

	nodeKR, err := link.GetKeyRing(parentNodeKR, protonDrive.AddrKR)
	if err != nil {
		return nil, err
	}

	return nodeKR, nil
}

func (protonDrive *ProtonDrive) getLink(ctx context.Context, linkID string) (*proton.Link, error) {
	if linkID == "" {
		// this is only possible when doing rootLink's parent, which should be handled beforehand
		return nil, ErrWrongUsageOfGetLink
	}

	// attempt to get from cache first
	if data := protonDrive.cache._get(linkID); data != nil && data.link != nil {
		return data.link, nil
	}

	// no cached data, fetch
	link, err := protonDrive.c.GetLink(ctx, protonDrive.MainShare.ShareID, linkID)
	if err != nil {
		return nil, err
	}

	// populate cache
	protonDrive.cache._insert(linkID, &link, nil)

	return &link, nil
}

func (protonDrive *ProtonDrive) getLinkKR(ctx context.Context, link *proton.Link) (*crypto.KeyRing, error) {
	if protonDrive.cache.disableCaching {
		return protonDrive._getLinkKR(ctx, link)
	}

	if link == nil {
		return nil, ErrWrongUsageOfGetLinkKR
	}

	// attempt to get from cache first
	if data := protonDrive.cache._get(link.LinkID); data != nil && data.link != nil {
		if data.kr != nil {
			return data.kr, nil
		}

		// decrypt keyring and cache it
		parentNodeKR, err := protonDrive.getLinkKRByID(ctx, data.link.ParentLinkID)
		if err != nil {
			return nil, err
		}

		kr, err := data.link.GetKeyRing(parentNodeKR, protonDrive.AddrKR)
		if err != nil {
			return nil, err
		}
		data.kr = kr
		return data.kr, nil
	}

	// no cached data, fetch
	protonDrive.cache._insert(link.LinkID, link, nil)

	return protonDrive.getLinkKR(ctx, link)
}

func (protonDrive *ProtonDrive) getLinkKRByID(ctx context.Context, linkID string) (*crypto.KeyRing, error) {
	if protonDrive.cache.disableCaching {
		return protonDrive._getLinkKRByID(ctx, linkID)
	}

	if linkID == "" {
		return protonDrive.MainShareKR, nil
	}

	// attempt to get from cache first
	if data := protonDrive.cache._get(linkID); data != nil && data.link != nil {
		return protonDrive.getLinkKR(ctx, data.link)
	}

	// log.Println("Not from cache")
	// no cached data, fetch
	link, err := protonDrive.getLink(ctx, linkID)
	if err != nil {
		return nil, err
	}

	return protonDrive.getLinkKR(ctx, link)
}

// TODO: handle removal upon rmdir, mv, etc. cases

// func (protonDrive *ProtonDrive) clearCache() {
// 	if protonDrive.cache.disableCaching {
// 		return
// 	}

// 	protonDrive.cache.Lock()
// 	defer protonDrive.cache.Unlock()

// 	for _, entry := range protonDrive.cache.data {
// 		entry.kr.ClearPrivateParams()
// 	}

// 	protonDrive.cache.data = make(map[string]*cacheEntry)
// }
