package certs

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewManager(t *testing.T) {
	// 测试证书管理器创建
	mgr, err := NewManager()
	assert.NoError(t, err)
	assert.NotNil(t, mgr)
	assert.NotNil(t, mgr.CACert)
	assert.NotNil(t, mgr.CAKey)

	// 测试CA证书导出
	tmpFile := "test_ca.pem"
	err = mgr.ExportCACert(tmpFile)
	assert.NoError(t, err)
	_, err = os.Stat(tmpFile)
	assert.False(t, os.IsNotExist(err))

	// 清理
	os.Remove(tmpFile)
}
