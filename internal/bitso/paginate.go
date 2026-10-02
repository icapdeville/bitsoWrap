package bitso

import (
	"strconv"
	"time"
)

const pageLimit = 100

// PageDelay es la pausa entre páginas para no topar el rate limit de Bitso.
var PageDelay = 300 * time.Millisecond

// PageFunc recibe cada página. Devuelve false para detener la paginación
// (por ejemplo, al encontrar un registro que ya está guardado).
type PageFunc[T any] func(page []T) (bool, error)

// paginate recorre un endpoint de Bitso usando "marker". En cada vuelta pide
// limit=100 y usa el id del último elemento como marker de la siguiente.
func paginate[T any](params map[string]string, fetch func(map[string]string) ([]T, error), id func(T) string, fn PageFunc[T]) error {
	p := make(map[string]string, len(params)+1)
	for k, v := range params {
		p[k] = v
	}
	p["limit"] = strconv.Itoa(pageLimit)

	for first := true; ; first = false {
		if !first && PageDelay > 0 {
			time.Sleep(PageDelay)
		}

		page, err := fetch(p)
		if err != nil {
			return err
		}
		if len(page) == 0 {
			return nil
		}

		cont, err := fn(page)
		if err != nil || !cont {
			return err
		}
		if len(page) < pageLimit {
			return nil
		}
		p["marker"] = id(page[len(page)-1])
	}
}

// WalkUserTrades recorre /user_trades página por página.
// Para traer solo trades nuevos usa params {"sort": "asc", "marker": "<último tid>"}.
// Sin marker y con sort por defecto (desc) recorre todo el historial, del más nuevo al más viejo.
func (c *BitsoClient) WalkUserTrades(params map[string]string, fn PageFunc[UserTrade]) error {
	return paginate(params, func(p map[string]string) ([]UserTrade, error) {
		resp, err := c.ListUserTrades(p)
		if err != nil {
			return nil, err
		}
		return resp.Payload, nil
	}, func(t UserTrade) string { return string(t.Tid) }, fn)
}

// WalkFundings recorre /fundings del más nuevo al más viejo (Bitso solo pagina hacia atrás).
// Para sincronizar, detén la paginación al encontrar un fid ya guardado.
func (c *BitsoClient) WalkFundings(params map[string]string, fn PageFunc[Funding]) error {
	return paginate(params, func(p map[string]string) ([]Funding, error) {
		resp, err := c.ListFundings(p)
		if err != nil {
			return nil, err
		}
		return resp.Payload, nil
	}, func(f Funding) string { return f.Fid }, fn)
}

// WalkWithdrawals recorre /withdrawals del más nuevo al más viejo.
func (c *BitsoClient) WalkWithdrawals(params map[string]string, fn PageFunc[Withdrawal]) error {
	return paginate(params, func(p map[string]string) ([]Withdrawal, error) {
		resp, err := c.ListWithdrawals(p)
		if err != nil {
			return nil, err
		}
		return resp.Payload, nil
	}, func(w Withdrawal) string { return w.Wid }, fn)
}
