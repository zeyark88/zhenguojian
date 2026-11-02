package fqsign

// Simon 分组密码（128 位密钥、64 位块、72 轮），x-argus 用。
// 逐字节对照 fqapp rust/src/sign/simon.rs 移植。

const simonZ uint64 = 0x3DC94C3A046D678B

func rotr64(v uint64, n uint) uint64 { return v>>n | v<<(64-n) }
func rotl64(v uint64, n uint) uint64 { return v<<n | v>>(64-n) }

// SimonKeyExpansion 密钥扩展（key 前 4 个字已填好）。
func SimonKeyExpansion(key *[72]uint64) {
	for i := 4; i < 72; i++ {
		tmp := rotr64(key[i-1], 3)
		tmp ^= key[i-3]
		tmp ^= rotr64(tmp, 1)
		notVal := ^key[i-4]
		bitVal := (simonZ >> uint((i-4)%62)) & 1
		key[i] = notVal ^ tmp ^ bitVal ^ uint64(3)
	}
}

// SimonEnc 加密单个 128 位块（仅标准模式，Argus 只用这个）。
func SimonEnc(pt [2]uint64, k [4]uint64) [2]uint64 {
	var key [72]uint64
	key[0], key[1], key[2], key[3] = k[0], k[1], k[2], k[3]
	SimonKeyExpansion(&key)

	xi, xi1 := pt[0], pt[1]
	for _, roundKey := range key {
		tmp := xi1
		f := rotl64(xi1, 1) & rotl64(xi1, 8)
		xi1 = xi ^ f ^ rotl64(xi1, 2) ^ roundKey
		xi = tmp
	}
	return [2]uint64{xi, xi1}
}
