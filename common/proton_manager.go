package common

import (
	"github.com/henrybear327/go-proton-api"
)

func getProtonManager(appVersion string) *proton.Manager {
	/* Notes on API calls: if the app version is not specified, the api calls will be rejected. */
	options := []proton.Option{
		proton.WithAppVersion(appVersion),
	}
	m := proton.New(options...)

	return m
}
