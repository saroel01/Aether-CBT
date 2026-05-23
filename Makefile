.PHONY: run seed build clean help

# Legacy Make targets (npm run dev is recommended)
run:
	go run cmd/server/main.go

seed:
	go run cmd/seed/main.go

build:
	go build -o bin/aether-cbt cmd/server/main.go

clean:
	rm -rf bin/ data/cbt_aether.db

help:
	@echo "Aether CBT - Available commands:"
	@echo ""
	@echo "  npm run dev              → Start BOTH backend + frontend (recommended)"
	@echo "  npm run dev:backend-only → Start only Go backend"
	@echo "  npm run dev:frontend-only→ Start only SvelteKit frontend"
	@echo "  npm run seed             → Seed sample data"
	@echo ""
	@echo "  make run                 → Start backend only (alternative)"
	@echo "  make seed                → Seed sample data"
	@echo "  make clean               → Clean build artifacts"
