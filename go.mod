module github.com/henrybear327/Proton-API-Bridge

go 1.22.0

toolchain go1.23.5

require (
	github.com/ProtonMail/gluon v0.17.1-0.20230724134000-308be39be96e
	github.com/ProtonMail/go-proton-api v0.4.1-0.20250121114701-67bd01ad0bc3
	github.com/ProtonMail/gopenpgp/v2 v2.8.2
	github.com/relvacode/iso8601 v1.6.0
	golang.org/x/sync v0.10.0
)

require (
	github.com/ProtonMail/bcrypt v0.0.0-20211005172633-e235017c1baf // indirect
	github.com/ProtonMail/go-crypto v1.1.5 // indirect
	github.com/ProtonMail/go-mime v0.0.0-20230322103455-7d82a3887f2f // indirect
	github.com/ProtonMail/go-srp v0.0.7 // indirect
	github.com/PuerkitoBio/goquery v1.8.1 // indirect
	github.com/andybalholm/cascadia v1.3.3 // indirect
	github.com/bradenaw/juniper v0.15.3 // indirect
	github.com/cloudflare/circl v1.5.0 // indirect
	github.com/cronokirby/saferith v0.33.0 // indirect
	github.com/emersion/go-message v0.18.2 // indirect
	github.com/emersion/go-vcard v0.0.0-20241024213814-c9703dde27ff // indirect
	github.com/go-resty/resty/v2 v2.16.3 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/henrybear327/go-proton-api v1.0.0 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/sirupsen/logrus v1.9.3 // indirect
	gitlab.com/c0b/go-ordered-json v0.0.0-20201030195603-febf46534d5a // indirect
	golang.org/x/crypto v0.32.0 // indirect
	golang.org/x/exp v0.0.0-20250106191152-7588d65b2ba8 // indirect
	golang.org/x/net v0.34.0 // indirect
	golang.org/x/sys v0.29.0 // indirect
	golang.org/x/text v0.21.0 // indirect
)

replace github.com/ProtonMail/go-proton-api => github.com/henrybear327/go-proton-api v0.0.0-20250127204557-9ee38cb0a689

replace github.com/go-resty/resty/v2 => github.com/LBeernaertProton/resty/v2 v2.0.0-20231129100320-dddf8030d93a
