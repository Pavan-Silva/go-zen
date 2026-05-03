package zen

import (
	"fmt"
	"io"
	"net/http"

	"google.golang.org/protobuf/proto"
)

// BindProtoBuf binds the request body as Protocol Buffer message into the provided proto.Message.
// The dest must implement proto.Message interface.
//
// Example:
//
//	var msg mypb.MyMessage
//	if err := c.BindProtoBuf(&msg); err != nil {
//	    c.Error(http.StatusBadRequest, "invalid protobuf")
//	    return
//	}
func (c *Context) BindProtoBuf(dest proto.Message) error {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		return err
	}
	defer c.Request.Body.Close()

	if err := proto.Unmarshal(body, dest); err != nil {
		return fmt.Errorf("protobuf: %w", err)
	}

	return nil
}

// ProtoBuf sends a Protocol Buffer response with the given status code.
//
// Example:
//
//	msg := &mypb.MyMessage{Data: "hello"}
//	c.ProtoBuf(http.StatusOK, msg)
func (c *Context) ProtoBuf(status int, msg proto.Message) {
	data, err := proto.Marshal(msg)
	if err != nil {
		http.Error(c.Response, "protobuf encode error", http.StatusInternalServerError)
		return
	}

	c.Response.Header().Set("Content-Type", "application/x-protobuf")
	c.Response.WriteHeader(status)
	c.Response.Write(data)
}
