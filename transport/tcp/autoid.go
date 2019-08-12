/*
-------------------------------------------------
   Author :       Zhang Fan
   date：         2019/5/19
   Description :
-------------------------------------------------
*/

package tcp

import (
    "sync/atomic"
)

var defaultAutoNum = &AutoNum{}

type AutoNum struct {
    id uint64
}

func (m *AutoNum) NextNum() uint64 {
    return atomic.AddUint64(&m.id, 1)
}

func AutoNextNum() uint64 {
    return atomic.AddUint64(&defaultAutoNum.id, 1)
}
