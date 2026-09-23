# Stratum agent instructions

Stratum is a production, self-hosted architecture workspace. Preserve backend authorization, exact structured design data, and security boundaries when changing it. Read `backend/AGENTS.md` or `ui/AGENTS.md` before modifying those services.

Put temporary AI working notes, plans, scratch research, and generated context documents only in the repository-root `ai-docs/` directory. It is ignored by Git and must not be used for product documentation, release notes, or decisions that contributors need to review. Durable user and maintainer documentation belongs in `README.md`, `docs/`, or code comments where appropriate. Never put credentials, customer data, or private design content in `ai-docs/`.

Before submitting a change, add focused tests for behavior that could regress, run the relevant backend and UI checks, and inspect the diff for secrets and unintended files.
