package proton_api_bridge

import (
	"context"
	"sync"

	"github.com/henrybear327/go-proton-api"
)

type linkCache struct {
	data               map[string]*proton.Link
	disableLinkCaching bool

	sync.RWMutex
}

func newLinkCache(disableLinkCaching bool) *linkCache {
	return &linkCache{
		data:               make(map[string]*proton.Link),
		disableLinkCaching: disableLinkCaching,
	}
}

func (linkCache *linkCache) _getLink(linkID string) *proton.Link {
	if linkCache.disableLinkCaching {
		return nil
	}

	linkCache.RLock()
	defer linkCache.RUnlock()

	if link, ok := linkCache.data[linkID]; ok {
		return link
	}
	return nil
}

func (linkCache *linkCache) _insertLink(linkID string, link *proton.Link) {
	if linkCache.disableLinkCaching {
		return
	}

	linkCache.Lock()
	defer linkCache.Unlock()

	linkCache.data[linkID] = link
}

func (protonDrive *ProtonDrive) getLink(ctx context.Context, linkID string) (*proton.Link, error) {
	// attempt to get from cache first
	if link := protonDrive.linkCache._getLink(linkID); link != nil {
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
	protonDrive.linkCache._insertLink(linkID, &link)

	return &link, nil
}
