package board

import (
	"database/sql"
	"errors"
	"sort"
)

func orderedCategories(q queryer) ([]Category, error) {
	cs, e := list[Category](q, "categories")
	sort.Slice(cs, func(i, j int) bool {
		if cs[i].SortOrder == cs[j].SortOrder {
			return cs[i].ID < cs[j].ID
		}
		return cs[i].SortOrder < cs[j].SortOrder
	})
	return cs, e
}
func defaultCategory(q queryer) (int64, error) {
	cs, e := orderedCategories(q)
	if e != nil {
		return 0, e
	}
	if len(cs) == 0 {
		return 0, errors.New("カテゴリがありません")
	}
	return cs[0].ID, nil
}

// Upgrade the former virtual category once, including when restoring old backups.
func normalizeCategories(q queryer) error {
	cs, e := orderedCategories(q)
	if e != nil {
		return e
	}
	var version string
	err := q.QueryRow("SELECT value FROM metadata WHERE key='category_model'").Scan(&version)
	if err != nil && err != sql.ErrNoRows {
		return err
	}
	if len(cs) == 0 || err == sql.ErrNoRows {
		r, e := q.Exec("INSERT INTO categories(data) VALUES('{}')")
		if e != nil {
			return e
		}
		id, e := r.LastInsertId()
		if e != nil {
			return e
		}
		order := 0
		if len(cs) > 0 {
			order = cs[0].SortOrder - 1
		}
		c := Category{ID: id, Name: "未分類", Color: "#fffdf8", TextColor: "#302d25", SortOrder: order}
		if e = put(q, "categories", id, c); e != nil {
			return e
		}
	}
	cs, e = orderedCategories(q)
	if e != nil {
		return e
	}
	first := cs[0]
	first.Dormant = false
	if e = put(q, "categories", first.ID, first); e != nil {
		return e
	}
	for _, table := range []string{"tasks", "series"} {
		if _, e = q.Exec("UPDATE "+table+" SET data=json_set(data,'$.category_id',?,'$.version',COALESCE(json_extract(data,'$.version'),0)+1) WHERE COALESCE(json_extract(data,'$.category_id'),0)=0", first.ID); e != nil {
			return e
		}
		var invalid int
		if e = q.QueryRow("SELECT count(*) FROM " + table + " WHERE json_extract(data,'$.category_id') NOT IN (SELECT id FROM categories)").Scan(&invalid); e != nil {
			return e
		}
		if invalid > 0 {
			return errors.New("タスクまたは定期設定のカテゴリが見つかりません")
		}
	}
	_, e = q.Exec("INSERT OR REPLACE INTO metadata(key,value) VALUES('category_model','1')")
	return e
}
