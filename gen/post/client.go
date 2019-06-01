// Code generated by goa v3.0.2, DO NOT EDIT.
//
// Post client
//
// Command:
// $ goa gen github.com/eniehack/persona-server/design

package post

import (
	"context"

	goa "goa.design/goa/v3/pkg"
)

// Client is the "Post" service client.
type Client struct {
	CreateEndpoint    goa.Endpoint
	ReferenceEndpoint goa.Endpoint
	DeleteEndpoint    goa.Endpoint
}

// NewClient initializes a "Post" service client given the endpoints.
func NewClient(create, reference, delete_ goa.Endpoint) *Client {
	return &Client{
		CreateEndpoint:    create,
		ReferenceEndpoint: reference,
		DeleteEndpoint:    delete_,
	}
}

// Create calls the "create" endpoint of the "Post" service.
func (c *Client) Create(ctx context.Context, p *NewPostPayload) (err error) {
	_, err = c.CreateEndpoint(ctx, p)
	return
}

// Reference calls the "reference" endpoint of the "Post" service.
func (c *Client) Reference(ctx context.Context, p *Post) (err error) {
	_, err = c.ReferenceEndpoint(ctx, p)
	return
}

// Delete calls the "delete" endpoint of the "Post" service.
func (c *Client) Delete(ctx context.Context, p *DeletePostPayload) (err error) {
	_, err = c.DeleteEndpoint(ctx, p)
	return
}
