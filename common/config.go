package common

import "os"

type Config struct {
	/* Constants */
	AppVersion string

	/* Login */
	FirstLoginCredential *FirstLoginCredentialData
	ReusableCredential   *ReusableCredentialData
	UseReusableLogin     bool
	CredentialCacheFile  string // If CredentialCacheFile is empty, no credential will be logged

	/* Setting */
	DestructiveIntegrationTest     bool // CAUTION: the integration test requires a clean proton drive
	EmptyTrashAfterIntegrationTest bool // CAUTION: the integration test will clean up all the data in the trash

	/* Drive */
	DataFolderName string
}

type FirstLoginCredentialData struct {
	Username string
	Password string
	TwoFA    string
}

type ReusableCredentialData struct {
	UID           string
	AccessToken   string
	RefreshToken  string
	SaltedKeyPass string // []byte <-> base64
}

func NewConfigWithDefaultValues() *Config {
	return &Config{
		AppVersion: "",

		FirstLoginCredential: &FirstLoginCredentialData{
			Username: "",
			Password: "",
			TwoFA:    "",
		},
		ReusableCredential: &ReusableCredentialData{
			UID:           "",
			AccessToken:   "",
			RefreshToken:  "",
			SaltedKeyPass: "", // []byte <-> base64
		},
		UseReusableLogin:    false,
		CredentialCacheFile: "",

		DestructiveIntegrationTest:     false,
		EmptyTrashAfterIntegrationTest: false,

		DataFolderName: "data",
	}
}

func NewConfigForIntegrationTests() *Config {
	appVersion := os.Getenv("PROTON_API_BRIDGE_APP_VERSION")
	username := os.Getenv("PROTON_API_BRIDGE_TEST_USERNAME")
	password := os.Getenv("PROTON_API_BRIDGE_TEST_PASSWORD")
	twoFA := os.Getenv("PROTON_API_BRIDGE_TEST_TWOFA")

	useReusableLoginStr := os.Getenv("PROTON_API_BRIDGE_TEST_USE_REUSABLE_LOGIN")
	useReusableLogin := false
	if useReusableLoginStr == "1" {
		useReusableLogin = true
	}

	uid := os.Getenv("PROTON_API_BRIDGE_TEST_UID")
	accessToken := os.Getenv("PROTON_API_BRIDGE_TEST_ACCESS_TOKEN")
	refreshToken := os.Getenv("PROTON_API_BRIDGE_TEST_REFRESH_TOKEN")
	saltedKeyPass := os.Getenv("PROTON_API_BRIDGE_TEST_SALTEDKEYPASS")

	return &Config{
		AppVersion: appVersion,

		FirstLoginCredential: &FirstLoginCredentialData{
			Username: username,
			Password: password,
			TwoFA:    twoFA,
		},
		ReusableCredential: &ReusableCredentialData{
			UID:           uid,
			AccessToken:   accessToken,
			RefreshToken:  refreshToken,
			SaltedKeyPass: saltedKeyPass, // []byte <-> base64
		},
		UseReusableLogin:    useReusableLogin,
		CredentialCacheFile: ".credential",

		DestructiveIntegrationTest:     true,
		EmptyTrashAfterIntegrationTest: true,

		DataFolderName: "data",
	}
}
