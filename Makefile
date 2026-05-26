.PHONY: help dev stop logs status seed psql reset clean

API_URL ?= http://localhost:8080

help:
	@echo "Hotel-booking dev orchestration"
	@echo ""
	@echo "  make dev      Start everything: docker infra + migrations + API + worker + both web apps"
	@echo "  make stop     Stop API/worker/web (PIDs in .dev/), leaves docker infra running"
	@echo "  make logs     tail -f all .dev/*.log"
	@echo "  make status   Show which dev processes are running"
	@echo "  make seed     Create a demo hotel + room type + published landing, flip to live"
	@echo "  make psql     Open psql against the dev database"
	@echo "  make reset    Drop docker volumes + restart infra + re-apply migrations (destroys data)"
	@echo "  make clean    Stop everything including docker infra"
	@echo ""
	@echo "URLs after 'make dev':"
	@echo "  API:      http://localhost:8080  (health: /healthz, /readyz)"
	@echo "  Admin:    http://localhost:3001  (signup → onboarding wizard)"
	@echo "  Booking:  http://localhost:3000  (guest landing at /:slug)"
	@echo "  MinIO:    http://localhost:9001  (console, user: minioadmin / minioadmin)"

dev:
	@mkdir -p .dev
	@echo "▸ docker compose up"
	@docker compose up -d
	@echo "▸ waiting for postgres"
	@./scripts/wait-for-postgres.sh
	@echo "▸ applying migrations"
	@$(MAKE) -C apps/api migrate-up
	@echo "▸ installing web deps (if needed)"
	@pnpm install --silent
	@echo "▸ starting API + worker + web apps"
	@./scripts/dev-start.sh
	@echo ""
	@echo "✓ everything up. URLs:"
	@echo "  API:      http://localhost:8080"
	@echo "  Admin:    http://localhost:3001"
	@echo "  Booking:  http://localhost:3000"
	@echo "  Logs:     make logs   (or tail -f .dev/*.log)"
	@echo "  Demo:     make seed   (creates a hotel + landing you can browse)"

stop:
	@./scripts/dev-stop.sh

clean: stop
	@docker compose down
	@echo "✓ docker infra stopped"

logs:
	@tail -F .dev/*.log

status:
	@./scripts/dev-status.sh

seed:
	@./scripts/seed-demo.sh

psql:
	@docker compose exec postgres psql -U hotel hotel_booking

reset:
	@./scripts/dev-stop.sh
	@docker compose down -v
	@$(MAKE) dev
