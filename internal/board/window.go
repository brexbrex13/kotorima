package board

import "encoding/json"

type WindowBounds struct {
	X      int `json:"x"`
	Y      int `json:"y"`
	Width  int `json:"width"`
	Height int `json:"height"`
}

func (s *Store) WindowBounds() (WindowBounds, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var raw string
	var v WindowBounds
	e := s.db.QueryRow("SELECT value FROM metadata WHERE key='window_bounds'").Scan(&raw)
	ok := e == nil && json.Unmarshal([]byte(raw), &v) == nil && validWindowBounds(v)
	return v, ok
}

func validWindowBounds(v WindowBounds) bool {
	return v.Width >= 360 && v.Height >= 420 && v.Width <= 20000 && v.Height <= 20000 && v.X >= -200000 && v.X <= 200000 && v.Y >= -200000 && v.Y <= 200000
}

func (s *Store) SaveWindowBounds(v WindowBounds) error {
	if !validWindowBounds(v) {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	b, e := json.Marshal(v)
	if e != nil {
		return e
	}
	_, e = s.db.Exec("INSERT INTO metadata(key,value) VALUES('window_bounds',?) ON CONFLICT(key) DO UPDATE SET value=excluded.value", string(b))
	return e
}
