/*
-------------------------------------------------
   Author :       Zhang Fan
   date：         2019/8/11
   Description :
-------------------------------------------------
*/

package tcp

import "context"

type EncodeRequestFunc func(ctx context.Context, request interface{}) ([]byte, error)
type DecodeRequestFunc func(ctx context.Context, data []byte) (interface{}, error)
type EncodeResponseFunc func(ctx context.Context, response interface{}) ([]byte, error)
type DecodeResponseFunc func(ctx context.Context, data []byte) (interface{}, error)
