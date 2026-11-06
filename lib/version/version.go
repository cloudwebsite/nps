package version

// VERSION 为产品展示版本（可递增，仅数字点号，便于比较与打标签）。
// 融合主干：在 yisier 0.26.40 血缘上叠加无损升级能力。
// 0.26.41：本族 npc 起具备挑战握手能力；第四段区分本仓构建；CoreVersion 仍锁定 0.26.0。
const VERSION = "0.26.41.3"

// CoreVersion 是 Bridge 握手使用的核心协议版本。
// 旧 yisier npc 依赖该值；在存量客户端清完前禁止抬高。
const CoreVersion = "0.26.0"

// GetVersion 返回核心协议版本，供握手 MD5 交换使用。
func GetVersion() string {
	return CoreVersion
}
