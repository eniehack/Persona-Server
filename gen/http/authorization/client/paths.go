// Code generated by goa v3.0.2, DO NOT EDIT.
//
// HTTP request path constructors for the Authorization service.
//
// Command:
// $ goa gen github.com/eniehack/persona-server/design

package client

// LoginAuthorizationPath returns the URL path to the Authorization service login HTTP endpoint.
func LoginAuthorizationPath() string {
	return "/auth/signature"
}

// RegisterAuthorizationPath returns the URL path to the Authorization service register HTTP endpoint.
func RegisterAuthorizationPath() string {
	return "/auth/new"
}
