package mcp

import (
	"bytes"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"io"
	"net/http"

	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	hostedListRequestHeader = "X-Compozy-Hosted-List-Request"
	hostedListPageLimit     = 128
)

type hostedListPage struct {
	revision uint64
	cursor   string
	next     string
}

type hostedListDelivery struct {
	revision       uint64
	delivered      uint64
	cursorRevision uint64
	cursor         string
	pages          map[string]hostedListPage
}

func hostedListRequestIdentity(request *jsonrpc.Request) (*jsonrpc.Request, error) {
	raw, err := json.Marshal(request.ID.Raw())
	if err != nil {
		return nil, err
	}
	copied := *request
	copied.Extra = &sdkmcp.RequestExtra{Header: http.Header{hostedListRequestHeader: {hostedListID(raw)}}}
	return &copied, nil
}

func hostedListID(raw []byte) string {
	if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
		return ""
	}
	if raw[0] == '"' {
		var id string
		if json.Unmarshal(raw, &id) != nil {
			return ""
		}
		return "s:" + id
	}
	return "n:" + string(raw)
}

func (d *hostedListDelivery) record(req sdkmcp.Request, result sdkmcp.Result) {
	page, ok := result.(*sdkmcp.ListToolsResult)
	if !ok || req.GetExtra() == nil {
		return
	}
	id := req.GetExtra().Header.Get(hostedListRequestHeader)
	if id == "" || len(d.pages) >= hostedListPageLimit {
		return
	}
	request, ok := req.(*sdkmcp.ListToolsRequest)
	if !ok {
		return
	}
	cursor := ""
	if request.Params != nil {
		cursor = request.Params.Cursor
	}
	if d.pages == nil {
		d.pages = make(map[string]hostedListPage)
	}
	d.pages[id] = hostedListPage{revision: d.revision, cursor: cursor, next: page.NextCursor}
}

func (d *hostedListDelivery) written(id string) bool {
	page, ok := d.pages[id]
	if !ok {
		return false
	}
	delete(d.pages, id)
	if page.revision != d.revision ||
		(page.cursor != "" && (d.cursorRevision != page.revision || d.cursor != page.cursor)) {
		return false
	}
	d.cursor, d.cursorRevision = page.next, page.revision
	if page.next != "" || d.delivered == page.revision {
		return false
	}
	d.delivered = page.revision
	return true
}

type hostedListOutput struct {
	io.Writer
	catalog *hostedProxyCatalog
}

func (w hostedListOutput) Write(data []byte) (int, error) {
	n, err := w.Writer.Write(data)
	if err == nil && n == len(data) {
		w.catalog.listResponsesWritten(data)
	}
	return n, err
}

func (c *hostedProxyCatalog) listResponsesWritten(data []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.delivery.pages) == 0 {
		return
	}
	type response struct {
		ID     jsontext.Value `json:"id"`
		Error  jsontext.Value `json:"error"`
		Method string         `json:"method"`
	}
	var responses []response
	if bytes.HasPrefix(bytes.TrimSpace(data), []byte("[")) {
		if json.Unmarshal(data, &responses) != nil {
			return
		}
	} else {
		var item response
		if json.Unmarshal(data, &item) != nil {
			return
		}
		responses = []response{item}
	}
	for _, item := range responses {
		if item.Method != "" {
			continue
		}
		id := hostedListID(item.ID)
		if len(item.Error) > 0 {
			delete(c.delivery.pages, id)
			continue
		}
		if c.delivery.written(id) {
			c.signal()
		}
	}
}
