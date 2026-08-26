// MySQL 存储。
//
// 设计要点（调研报告 §6.3）：
//   - price_snapshots **只追加不更新**。历史变价是本项目相对竞品的核心差异，
//     绝不能被覆盖掉。唯一索引挡重复，重跑当天用 INSERT ... ON DUPLICATE KEY UPDATE。
//   - raw_price **永久保留**。实测单个 SKU 就有 78 种格式，解析器一定还会遇到第 79 种，
//     出错时这是唯一的排查依据。
//   - 金额一律存**最小货币单位的整数**，不用浮点。
//   - 汇率逐日存档，历史价格配历史汇率。
package main

import (
	"database/sql"
	"fmt"
	"time"
)

const schema = `
CREATE TABLE IF NOT EXISTS price_snapshots (
	id            BIGINT AUTO_INCREMENT PRIMARY KEY,
	date          DATE         NOT NULL,
	product_id    VARCHAR(32)  NOT NULL,
	storefront    VARCHAR(8)   NOT NULL,
	currency      VARCHAR(8)   NOT NULL,
	display_name  VARCHAR(191) NOT NULL COMMENT 'Apple 原始展示名',
	raw_price     VARCHAR(64)  NOT NULL COMMENT 'Apple 原始格式化价格串，排查解析问题的唯一依据',
	tier          VARCHAR(32)  NULL COMMENT '归一化档位，unclassified 时为 NULL',
	period        VARCHAR(16)  NULL,
	amount_minor  BIGINT       NULL COMMENT '本地币种最小单位',
	usd_micro     BIGINT       NULL COMMENT '折算美元的百万分之一，避免浮点',
	parse_status  VARCHAR(24)  NOT NULL COMMENT 'ok / unclassified / unknown_sku / parse_error',
	note          VARCHAR(255) NULL,
	created_at    TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP,
	UNIQUE KEY uniq_snapshot (date, product_id, storefront, display_name, raw_price),
	KEY idx_lookup (product_id, tier, period, date),
	KEY idx_storefront (storefront, date),
	KEY idx_status (parse_status, date)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS fx_rates (
	date       DATE        NOT NULL,
	base       VARCHAR(8)  NOT NULL,
	currency   VARCHAR(8)  NOT NULL,
	rate       DECIMAL(24,10) NOT NULL COMMENT '1 base = rate 单位 currency',
	source     VARCHAR(64) NOT NULL,
	created_at TIMESTAMP   NOT NULL DEFAULT CURRENT_TIMESTAMP,
	PRIMARY KEY (date, base, currency)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS crawl_runs (
	id           BIGINT AUTO_INCREMENT PRIMARY KEY,
	date         DATE      NOT NULL,
	started_at   TIMESTAMP NOT NULL,
	finished_at  TIMESTAMP NULL,
	ok_count     INT NOT NULL DEFAULT 0,
	absent_count INT NOT NULL DEFAULT 0,
	error_count  INT NOT NULL DEFAULT 0,
	status       VARCHAR(16) NOT NULL COMMENT 'running / success / aborted',
	note         TEXT NULL,
	KEY idx_date (date)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
`

// Store 封装数据库访问。
type Store struct{ db *sql.DB }

// OpenStore 连接 MySQL 并确保表结构存在。
func OpenStore(dsn string) (*Store, error) {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("mysql open: %w", err)
	}
	db.SetMaxOpenConns(10)
	db.SetConnMaxLifetime(time.Hour)
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("mysql ping: %w", err)
	}
	// schema 里有多条语句，逐条执行
	for _, stmt := range splitStatements(schema) {
		if _, err := db.Exec(stmt); err != nil {
			return nil, fmt.Errorf("建表失败: %w", err)
		}
	}
	return &Store{db: db}, nil
}

func (s *Store) Close() error { return s.db.Close() }

func splitStatements(s string) []string {
	var out []string
	for _, stmt := range splitOnSemicolon(s) {
		if t := trimSpace(stmt); t != "" {
			out = append(out, t)
		}
	}
	return out
}

// Snapshot 是要落库的一条价格记录。
type Snapshot struct {
	Date        string
	ProductID   string
	Storefront  string
	Currency    string
	DisplayName string
	RawPrice    string
	Tier        sql.NullString
	Period      sql.NullString
	AmountMinor sql.NullInt64
	USDMicro    sql.NullInt64
	ParseStatus string
	Note        sql.NullString
}

// SaveSnapshots 批量写入。重跑当天会覆盖同一条记录的解析结果
// （raw_price 是唯一键的一部分，所以原始数据本身不会丢）。
func (s *Store) SaveSnapshots(rows []Snapshot) error {
	if len(rows) == 0 {
		return nil
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT INTO price_snapshots
			(date, product_id, storefront, currency, display_name, raw_price,
			 tier, period, amount_minor, usd_micro, parse_status, note)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?)
		ON DUPLICATE KEY UPDATE
			tier=VALUES(tier), period=VALUES(period),
			amount_minor=VALUES(amount_minor), usd_micro=VALUES(usd_micro),
			parse_status=VALUES(parse_status), note=VALUES(note)`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, r := range rows {
		if _, err := stmt.Exec(r.Date, r.ProductID, r.Storefront, r.Currency,
			r.DisplayName, r.RawPrice, r.Tier, r.Period,
			r.AmountMinor, r.USDMicro, r.ParseStatus, r.Note); err != nil {
			return fmt.Errorf("写入 %s/%s/%s 失败: %w", r.ProductID, r.Storefront, r.DisplayName, err)
		}
	}
	return tx.Commit()
}

// SaveFX 存档某天的汇率表。
func (s *Store) SaveFX(fx *FXRates) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT INTO fx_rates (date, base, currency, rate, source)
		VALUES (?,?,?,?,?)
		ON DUPLICATE KEY UPDATE rate=VALUES(rate), source=VALUES(source)`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for cur, rate := range fx.Rates {
		if _, err := stmt.Exec(fx.Date, fx.Base, cur, rate, fx.Source); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// StartRun 记录一次抓取开始，返回 run id。
func (s *Store) StartRun(date string) (int64, error) {
	res, err := s.db.Exec(
		`INSERT INTO crawl_runs (date, started_at, status) VALUES (?, NOW(), 'running')`, date)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// FinishRun 更新抓取结果。
func (s *Store) FinishRun(id int64, status string, ok, absent, errCount int, note string) error {
	_, err := s.db.Exec(`
		UPDATE crawl_runs
		SET finished_at=NOW(), status=?, ok_count=?, absent_count=?, error_count=?, note=?
		WHERE id=?`, status, ok, absent, errCount, note, id)
	return err
}

// LatestDate 返回库里最新一天有 ok 数据的日期，没有则返回空串。
func (s *Store) LatestDate() (string, error) {
	var d sql.NullString
	err := s.db.QueryRow(
		`SELECT MAX(date) FROM price_snapshots WHERE parse_status='ok'`).Scan(&d)
	if err != nil {
		return "", err
	}
	if !d.Valid {
		return "", nil
	}
	return d.String[:10], nil
}
