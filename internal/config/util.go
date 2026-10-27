package config

import (
	"log"
	"os"
	"path/filepath"
	"runtime"

	"github.com/joho/godotenv"
)

// LoadEnvFromRoot 自动查找项目根目录（包含 go.mod 的目录）并加载 .env
func LoadEnvFromRoot() string {
	// 1. 获取当前调用文件的绝对路径
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		log.Fatal("无法获取当前文件路径")
	}

	// 2. 向上遍历目录，直到找到 go.mod 文件
	dir := filepath.Dir(filename)
	for {
		// 检查当前目录是否有 go.mod
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			// 找到根目录，尝试加载 .env
			envPath := filepath.Join(dir, ".env")
			if err := godotenv.Load(envPath); err != nil {
				// 如果 .env 不存在，godotenv 会报错，根据需求决定是否致命错误
				// 在生产环境中，.env 通常不存在，建议忽略错误或仅记录警告
				log.Printf("Warning: .env file not loaded or not found at %s: %v", envPath, err)
			} else {
				log.Printf("Successfully loaded .env from: %s", envPath)
				return envPath
			}
		}

		// 移动到父目录
		parent := filepath.Dir(dir)
		if parent == dir {
			// 已经到达文件系统根目录，仍未找到 go.mod
			log.Fatal("Could not find project root (go.mod not found)")
		}
		dir = parent
	}
}
