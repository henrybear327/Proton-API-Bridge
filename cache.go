package proton_api_bridge

import (
	"context"
	"sync"

	"github.com/henrybear327/go-proton-api"
)

type cacheEntry struct {
	link *proton.Link
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

func (linkCache *cache) _getLink(linkID string) *proton.Link {
	if linkCache.disableCaching {
		return nil
	}

	linkCache.RLock()
	defer linkCache.RUnlock()

	if data, ok := linkCache.data[linkID]; ok && data.link != nil {
		return data.link
	}
	return nil
}

func (linkCache *cache) _insertLink(linkID string, link *proton.Link) {
	if linkCache.disableCaching {
		return
	}

	linkCache.Lock()
	defer linkCache.Unlock()

	linkCache.data[linkID] = &cacheEntry{
		link: link,
	}
}

func (protonDrive *ProtonDrive) getLink(ctx context.Context, linkID string) (*proton.Link, error) {
	// attempt to get from cache first
	if link := protonDrive.cache._getLink(linkID); link != nil {
		// log.Println("From cache")
		return link, nil
	}

	// log.Println("Not from cache")
	// no cached data, fetch
	link, err := protonDrive.c.GetLink(ctx, protonDrive.MainShare.ShareID, linkID)
	if err != nil {
		return nil, err
	}

	// populate cache
	protonDrive.cache._insertLink(linkID, &link)

	return &link, nil
}
