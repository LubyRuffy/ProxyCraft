#!/bin/bash

# 设置颜色
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# 获取项目根目录的绝对路径
PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
WEB_DIR="${PROJECT_ROOT}/web"
API_DIST_DIR="${PROJECT_ROOT}/api/dist"

echo -e "${YELLOW}开始构建ProxyCraft Web界面${NC}"
echo -e "${YELLOW}项目根目录: ${PROJECT_ROOT}${NC}"

# 检查是否安装了Node.js
if ! command -v node &> /dev/null; then
    echo -e "${YELLOW}未检测到Node.js，请先安装Node.js${NC}"
    exit 1
fi

# 检查是否安装了npm
if ! command -v npm &> /dev/null; then
    echo -e "${YELLOW}未检测到npm，请先安装npm${NC}"
    exit 1
fi

# 确保目录存在
mkdir -p "${API_DIST_DIR}"

# 切换到web目录
cd "${WEB_DIR}" || {
    echo -e "${RED}无法进入web目录: ${WEB_DIR}${NC}"
    exit 1
}

# 安装依赖
echo -e "${YELLOW}安装前端依赖...${NC}"
npm install

# 构建前端项目
echo -e "${YELLOW}构建前端项目...${NC}"
npm run build

# 严格检查构建结果
BUILD_EXIT_CODE=$?
if [ $BUILD_EXIT_CODE -ne 0 ]; then
    echo -e "${RED}前端构建失败，构建命令返回代码: ${BUILD_EXIT_CODE}${NC}"
    exit 1
fi

# 检查构建是否成功 - 检查index.html是否存在及目录是否为空
if [ -f "${API_DIST_DIR}/index.html" ] && [ "$(ls -A "${API_DIST_DIR}")" ]; then
    echo -e "${GREEN}前端构建成功，构建结果位于: ${API_DIST_DIR}${NC}"
    echo -e "${GREEN}确认index.html文件已创建${NC}"
else
    echo -e "${RED}前端构建可能成功但未生成index.html或输出目录为空，请检查错误信息${NC}"
    echo -e "${RED}目录内容:${NC}"
    ls -la "${API_DIST_DIR}"
    exit 1
fi

# 返回到项目根目录
cd "${PROJECT_ROOT}" || exit 1

echo -e "${GREEN}构建完成，现在可以使用 ./ProxyCraft -mode web 启动Web模式${NC}" 