package proton_api_bridge

import (
	"context"

	"github.com/henrybear327/go-proton-api"
)

func (protonDrive *ProtonDrive) getLink(ctx context.Context, linkID string) (*proton.Link, error) {
	link, err := protonDrive.c.GetLink(ctx, protonDrive.MainShare.ShareID, linkID)
	if err != nil {
		return nil, err
	}

	return &link, nil
}
