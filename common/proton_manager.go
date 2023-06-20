package common

import (
	"github.com/henrybear327/go-proton-api"
)

// TODO: use proper appname and version
func AppVersion() string {
	// return "web-drive@5.0.13.8"
	return "ios-drive@1.14.0"
}

func defaultAPIOptions() []proton.Option {
	return []proton.Option{
		proton.WithAppVersion(AppVersion()),
	}
}

func getProtonManager() *proton.Manager {
	/*
		Notes on API calls

		If the app version is not specified, the api calls will be rejected.
	*/
	m := proton.New(defaultAPIOptions()...)

	return m
}
