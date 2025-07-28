#!/bin/bash
# 安装 Git hooks 脚本

set -e

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
NC='\033[0m'

echo -e "${BLUE}安装 Git hooks...${NC}"

# 检查是否在 Git 仓库中
if [ ! -d ".git" ]; then
    echo -e "${RED}错误: 当前目录不是 Git 仓库${NC}"
    exit 1
fi

# 创建 hooks 目录（如果不存在）
mkdir -p .git/hooks

# 复制 pre-commit hook
if [ -f ".githooks/pre-commit" ]; then
    cp .githooks/pre-commit .git/hooks/pre-commit
    chmod +x .git/hooks/pre-commit
    echo -e "${GREEN}Pre-commit hook 已安装${NC}"
else
    echo -e "${RED}错误: .githooks/pre-commit 文件不存在${NC}"
    exit 1
fi

echo -e "${GREEN}Git hooks 安装完成!${NC}"
echo -e "${BLUE}现在每次 git commit 时都会自动运行代码质量检查${NC}"
echo -e "${BLUE}如果需要跳过检查，可以使用: git commit --no-verify${NC}"