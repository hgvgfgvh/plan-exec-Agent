package experience

import (
	"AgentTest/util/rag"
	"context"
	"database/sql"
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sync"

	"github.com/mattn/go-sqlite3" // 必须显式引用以使用 sqlite3.SQLiteDriver
)

var (
	onceRegister sync.Once
	driverName   = "sqlite3_with_vec"
)

type SqliteExperienceManager struct {
	db     *sql.DB
	dbPath string
}

func NewSqliteExperienceManager(dbPath string) (*SqliteExperienceManager, error) {
	// 1. 注册驱动 (全局只需注册一次)
	// 驱动会自动寻找同级目录下的 sqlite-vec.dll
	onceRegister.Do(func() {
		sql.Register(driverName, &sqlite3.SQLiteDriver{
			Extensions: []string{
				"vec0",
				//TODO
				//sqlite向量化存储需要这个dll  C:\DATA\GODATA\AgentTest\vec0.dll
				//且后续Navicate中删除不需要的数据时 需要 开启一个查询窗口 然后运行一次 SELECT load_extension('C:\\DATA\\GODATA\\AgentTest\\sqlite-vec.dll'); 激活  ； 后再在同一个窗口中 DELETE FROM experience_meta WHERE id = 5;（需要移除SELECT load_extension('C:\\DATA\\GODATA\\AgentTest\\sqlite-vec.dll')）
			},
		})
	})

	// 2. 自动处理路径
	dir := filepath.Dir(dbPath)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("mkdir failed: %w", err)
		}
	}

	// 3. 使用注册的自定义驱动打开数据库
	db, err := sql.Open(driverName, dbPath)
	if err != nil {
		return nil, fmt.Errorf("open db with extension failed: %w", err)
	}

	// 4. 执行初始化
	initSQL := `
    CREATE TABLE IF NOT EXISTS experience_meta (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        query TEXT,
        skill_tree TEXT,
        created_at DATETIME DEFAULT CURRENT_TIMESTAMP
    );
    CREATE VIRTUAL TABLE IF NOT EXISTS experience_vec USING vec0(
        query_embedding float[1024]
    );
    -- 【核心：自动清理触发器】
    CREATE TRIGGER IF NOT EXISTS trg_delete_vec
    AFTER DELETE ON experience_meta
    BEGIN
        DELETE FROM experience_vec WHERE rowid = OLD.id;
    END;
    `

	if _, err := db.Exec(initSQL); err != nil {
		db.Close() // 初始化失败及时关闭连接
		return nil, fmt.Errorf("init schema failed: %w", err)
	}

	return &SqliteExperienceManager{
		db:     db,
		dbPath: dbPath,
	}, nil
}

// float32SliceToByte 将 []float32 转换为 sqlite-vec 需要的紧凑二进制 BLOB
func float32SliceToByte(slice []float32) []byte {
	buf := make([]byte, len(slice)*4)
	for i, f := range slice {
		binary.LittleEndian.PutUint32(buf[i*4:], math.Float32bits(f))
	}
	return buf
}

// StoreExperience 修复版本
func (m *SqliteExperienceManager) StoreExperience(ctx context.Context, query, skillTree string) error {
	client, err := rag.GetClient()
	if err != nil {
		return err
	}
	vec, err := client.GetEmbedding(ctx, query)
	if err != nil {
		return fmt.Errorf("get embedding failed: %w", err)
	}

	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	res, err := tx.ExecContext(ctx, "INSERT INTO experience_meta(query, skill_tree) VALUES(?, ?)", query, skillTree)
	if err != nil {
		return err
	}
	lastID, _ := res.LastInsertId()

	// 【关键修复】：将 float32 切片转换为二进制字节流，而不是 JSON 字符串
	vecBlob := float32SliceToByte(vec)
	_, err = tx.ExecContext(ctx, "INSERT INTO experience_vec(rowid, query_embedding) VALUES(?, ?)", lastID, vecBlob)
	if err != nil {
		return fmt.Errorf("vector insert failed: %w", err)
	}

	return tx.Commit()
}

// RetrieveExperience 修复版本
func (m *SqliteExperienceManager) RetrieveExperience(ctx context.Context, query string) (string, error) {
	client, err := rag.GetClient()
	if err != nil {
		return "", err
	}
	queryVec, err := client.GetEmbedding(ctx, query)
	if err != nil {
		return "", err
	}

	// 1. 将查询向量转为二进制 BLOB
	queryBlob := float32SliceToByte(queryVec)

	// 2. 执行检索
	// 我们需要从 meta 表拿 query 和 skill_tree，从 vec 表拿 distance
	// 阈值过滤：相似度 > 0.7 意味着 距离 < 0.3
	const similarityThreshold = 0.4
	const distanceThreshold = 1.0 - similarityThreshold

	rows, err := m.db.QueryContext(ctx, `
       SELECT m.query, m.skill_tree, v.distance
       FROM experience_meta m
       JOIN (
          SELECT rowid, distance
          FROM experience_vec
          WHERE query_embedding MATCH ?
          /* 注意：某些版本的 sqlite-vec 需要在此处指定 k=1 */
          ORDER BY distance
          LIMIT 1
       ) v ON m.id = v.rowid
       WHERE v.distance <= ?`, queryBlob, distanceThreshold)

	if err != nil {
		return "", fmt.Errorf("vector search failed: %w", err)
	}
	defer rows.Close()

	if rows.Next() {
		var matchedQuery string
		var skillTree string
		var distance float64

		if err := rows.Scan(&matchedQuery, &skillTree, &distance); err != nil {
			return "", err
		}

		// 3. 按照要求拼接返回： query:skillTree
		// 这里的 matchedQuery 是数据库里存的原始需求，skillTree 是对应的技能树
		result := fmt.Sprintf("%s:%s", matchedQuery, skillTree)

		// 调试日志（可选）：查看实际相似度
		fmt.Printf("[RAG] 匹配成功，相似度: %.2f\n", 1.0-distance)

		return result, nil
	}

	// 如果没有匹配到或距离超过阈值，返回空字符串
	return "", nil
}

// Close 提供关闭连接的方法
func (m *SqliteExperienceManager) Close() error {
	if m.db != nil {
		return m.db.Close()
	}
	return nil
}
