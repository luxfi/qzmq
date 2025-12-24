#!/bin/bash
# Script to publish QZMQ to GitHub

set -e

# Colors for output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

echo -e "${GREEN}=====================================${NC}"
echo -e "${GREEN}    QZMQ GitHub Publishing Script    ${NC}"
echo -e "${GREEN}=====================================${NC}"
echo

# Check if gh CLI is installed
if ! command -v gh &> /dev/null; then
    echo -e "${RED}GitHub CLI (gh) is not installed.${NC}"
    echo "Install it from: https://cli.github.com/"
    echo "Or use: brew install gh"
    exit 1
fi

# Check if logged in to GitHub
if ! gh auth status &> /dev/null; then
    echo -e "${YELLOW}Not logged in to GitHub. Running 'gh auth login'...${NC}"
    gh auth login
fi

# Repository information
REPO_NAME="qzmq"
REPO_DESC="Quantum-safe ZeroMQ transport with ML-KEM and ML-DSA"
REPO_TOPICS="zeromq post-quantum cryptography ml-kem ml-dsa quantum-safe golang"

echo -e "${YELLOW}Creating GitHub repository...${NC}"

# Create the repository
gh repo create luxfi/$REPO_NAME \
    --public \
    --description "$REPO_DESC" \
    --clone=false \
    --add-readme=false \
    || echo -e "${YELLOW}Repository may already exist, continuing...${NC}"

# Add remote if not exists
if ! git remote | grep -q origin; then
    git remote add origin https://github.com/luxfi/$REPO_NAME.git
else
    echo -e "${YELLOW}Remote 'origin' already exists${NC}"
fi

# Set upstream branch
git branch -M main

# Push to GitHub
echo -e "${YELLOW}Pushing to GitHub...${NC}"
git push -u origin main --force

# Set repository topics
echo -e "${YELLOW}Setting repository topics...${NC}"
gh repo edit luxfi/$REPO_NAME --add-topic zeromq
gh repo edit luxfi/$REPO_NAME --add-topic post-quantum
gh repo edit luxfi/$REPO_NAME --add-topic cryptography
gh repo edit luxfi/$REPO_NAME --add-topic quantum-safe
gh repo edit luxfi/$REPO_NAME --add-topic golang
gh repo edit luxfi/$REPO_NAME --add-topic ml-kem
gh repo edit luxfi/$REPO_NAME --add-topic ml-dsa

# Create initial release
echo -e "${YELLOW}Creating initial release v0.1.0...${NC}"

# Create and push tag
git tag -a v0.1.0 -m "Initial release of QZMQ

- Post-quantum secure transport for ZeroMQ
- ML-KEM (Kyber) and ML-DSA (Dilithium) support  
- Dual backend support (Pure Go and C bindings)
- Automatic key rotation
- 0-RTT resumption
- Comprehensive documentation"

git push origin v0.1.0

# Create GitHub release
gh release create v0.1.0 \
    --title "QZMQ v0.1.0 - Initial Release" \
    --notes "## Initial Release of QZMQ

QZMQ (QuantumZMQ) is a post-quantum secure transport layer for ZeroMQ that provides quantum-resistant encryption using NIST-standardized algorithms.

### Features

- **Post-Quantum Security**: ML-KEM (Kyber) and ML-DSA (Dilithium)
- **High Performance**: Hardware-accelerated AES-GCM
- 🔄 **Dual Backend**: Pure Go and C bindings support
- 🔑 **Hybrid Modes**: Classical + post-quantum algorithms
- **0-RTT Resumption**: Fast reconnection
- 🛡️ **DoS Protection**: Anti-DoS cookies
- 🔄 **Auto Key Rotation**: Time and volume-based

### 📦 Installation

\`\`\`bash
go get github.com/luxfi/qzmq@v0.1.0
\`\`\`

### Quick Start

\`\`\`go
import \"github.com/luxfi/qzmq\"

// Create transport with quantum-safe defaults
transport, _ := qzmq.New(qzmq.DefaultOptions())

// Create encrypted socket
socket, _ := transport.NewSocket(qzmq.REP)
socket.Bind(\"tcp://*:5555\")

// Send/receive with automatic encryption
msg, _ := socket.Recv()
socket.Send([]byte(\"Quantum-safe reply\"))
\`\`\`

### 📖 Documentation

See the [README](https://github.com/luxfi/qzmq/blob/main/README.md) for complete documentation and examples.

### Note

This is an initial release. ML-KEM and ML-DSA implementations are currently stubs pending integration with production quantum-safe libraries.

### 🙏 Credits

Built by [Lux Network](https://lux.network) for a quantum-safe future." \
    --prerelease

echo
echo -e "${GREEN}=====================================${NC}"
echo -e "${GREEN}    Publishing Complete!             ${NC}"
echo -e "${GREEN}=====================================${NC}"
echo
echo -e "Repository URL: ${GREEN}https://github.com/luxfi/qzmq${NC}"
echo -e "Installation:   ${GREEN}go get github.com/luxfi/qzmq@v0.1.0${NC}"
echo
echo "Next steps:"
echo "  1. Enable GitHub Actions in repository settings"
echo "  2. Add branch protection rules for 'main'"
echo "  3. Configure Codecov integration for coverage reports"
echo "  4. Add pkg.go.dev badge after first index"
echo
echo -e "${YELLOW}To update documentation on pkg.go.dev:${NC}"
echo "  curl https://sum.golang.org/lookup/github.com/luxfi/qzmq@v0.1.0"
echo "  curl https://proxy.golang.org/github.com/luxfi/qzmq/@v/v0.1.0.info"