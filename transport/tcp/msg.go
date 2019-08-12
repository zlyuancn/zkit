/*
-------------------------------------------------
   Author :       Zhang Fan
   date：         2019/8/11
   Description :
-------------------------------------------------
*/

package tcp

import (
    "errors"
)

const minRequestDataLength = 8 + 1
const minResponseDataLength = 8 + 2

type rawRequest struct {
    id   uint64
    rpc  string
    body []byte
}

func newRawRequest(id uint64, rpc string, body []byte) *rawRequest {
    return &rawRequest{
        id:   id,
        rpc:  rpc,
        body: body,
    }
}

func (m *rawRequest) Load(data []byte) (error) {
    if len(data) < minRequestDataLength {
        return errors.New("无效的数据")
    }

    m.id = BytesToUint64(data[:8])
    data = data[8:]

    le := data[0]
    data = data[1:]
    if le == 0 {
        return errors.New("数据头错误")
    }

    if len(data) < int(le) {
        return errors.New("不完整的数据")
    }

    m.rpc = string(data[:le])
    m.body = data[le:]
    return nil
}

func (m *rawRequest) Dump() []byte {
    rpcbs := []byte(m.rpc)
    out := make([]byte, 8+1+len(rpcbs)+len(m.body))
    bs := out

    copy(bs, Uint64ToBytes(m.id))
    bs = bs[8:]

    bs[0] = byte(len(rpcbs))
    bs = bs[1:]

    copy(bs, rpcbs)
    bs = bs[len(rpcbs):]

    if len(m.body) != 0 {
        copy(bs, m.body)
    }
    return out
}

type rawResponse struct {
    id   uint64
    err  error
    body []byte
}

func newRawResponse(id uint64, err error, body []byte) *rawResponse {
    return &rawResponse{
        id:   id,
        err:  err,
        body: body,
    }
}

func (m *rawResponse) Load(data []byte) (error) {
    if len(data) < minResponseDataLength {
        return errors.New("无效的数据")
    }

    m.id = BytesToUint64(data[:8])
    data = data[8:]

    le := BytesToUint16(data[:2])
    data = data[2:]
    if le == 0 {
        m.body = data
        return nil
    }

    if len(data) < int(le) {
        return errors.New("不完整的数据")
    }

    m.err = errors.New(string(data[:le]))
    m.body = data[le:]
    return nil
}
func (m *rawResponse) Dump() []byte {
    var errmsg []byte
    if m.err != nil {
        errmsg = []byte(m.err.Error())
    }

    // 此处不用bytes.Buffer
    out := make([]byte, 8+2+len(errmsg)+len(m.body))
    bs := out

    copy(bs, Uint64ToBytes(m.id))
    bs = bs[8:]

    copy(bs, Uint16ToBytes(uint16(len(errmsg))))
    bs = bs[2:]

    if m.err != nil {
        copy(bs, errmsg)
        bs = bs[len(errmsg):]
    }

    if len(m.body) != 0 {
        copy(bs, m.body)
    }
    return out
}
