package capabilities

// ExternalPackCatalog 外挂 skill_packs 包的第一层摘要（不含 SKILL.md 全文）。
type ExternalPackCatalog struct {
	ID          string
	Title       string
	Description string
	Summary     string
}

var (
	externalPackLister   func() []ExternalPackCatalog
	externalPackFullText func(id string) (string, bool)
)

// RegisterExternalPackCatalog 由 skillpacks.Apply 注册外挂包列举与全文读取。
func RegisterExternalPackCatalog(
	lister func() []ExternalPackCatalog,
	fullText func(id string) (string, bool),
) {
	externalPackLister = lister
	externalPackFullText = fullText
}

func snapshotExternalPacks() []ExternalPackCatalog {
	if externalPackLister == nil {
		return nil
	}
	return externalPackLister()
}

func externalPackDocument(id string) (string, bool) {
	if externalPackFullText == nil {
		return "", false
	}
	return externalPackFullText(id)
}
