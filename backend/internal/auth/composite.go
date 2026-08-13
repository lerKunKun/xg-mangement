package auth

import "net/http"

type Authenticator interface {
	Authenticate(*http.Request) (Principal, bool)
}

type CompositeAuthenticator []Authenticator

func (authenticators CompositeAuthenticator) Authenticate(request *http.Request) (Principal, bool) {
	for _, authenticator := range authenticators {
		if authenticator == nil {
			continue
		}
		if principal, ok := authenticator.Authenticate(request); ok {
			return principal, true
		}
	}
	return Principal{}, false
}
