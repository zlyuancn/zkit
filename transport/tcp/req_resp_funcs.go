/*
-------------------------------------------------
   Author :       Zhang Fan
   date：         2019/8/11
   Description :
-------------------------------------------------
*/

package tcp

import (
    "context"
)

type ServerRequestFunc func(context.Context, []byte) context.Context
type ServerResponseFunc func(context.Context, interface{}) context.Context
type ClientRequestFunc func(context.Context, interface{}) context.Context
type ClientResponseFunc func(context.Context, []byte) context.Context
