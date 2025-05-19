#!/bin/bash

# 设置颜色
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# 获取项目根目录的绝对路径
PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
API_DIST_DIR="${PROJECT_ROOT}/api/dist"

echo -e "${YELLOW}开始构建ProxyCraft${NC}"
echo -e "${YELLOW}项目根目录: ${PROJECT_ROOT}${NC}"

# 确保有dist目录
mkdir -p "${API_DIST_DIR}"

# 编译项目
echo -e "${YELLOW}编译项目...${NC}"
cd "${PROJECT_ROOT}" && go build -o ProxyCraft

# 检查编译是否成功
if [ $? -eq 0 ]; then
    echo -e "${GREEN}编译成功，可以使用以下命令运行：${NC}"
    echo -e "${PROJECT_ROOT}/ProxyCraft             # CLI模式"
    echo -e "${PROJECT_ROOT}/ProxyCraft -mode web   # Web界面模式"
    echo -e ""
    echo -e "${YELLOW}要构建Web前端，请运行：${NC}"
    echo -e "${PROJECT_ROOT}/build_web.sh"
else
    echo -e "${RED}编译失败，请检查错误信息${NC}"
    exit 1
fi 