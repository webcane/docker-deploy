# Comparison

docker-deploy is designed for a specific use case — deploying a local docker-compose project to a single VPS over SSH without git on the remote. Here is how it compares to other tools.

## Comparison Table

| Tool | Docker Compose native on remote | .env / secrets handling | Time to first deploy | Compose-centric design | SSH best practices (no root required) | Complexity / learning curve | Remote dependencies | Requires git on VPS | Best fit |
|------|----------------------------------|-------------------------|---------------------|------------------------|---------------------------------------|------------------------------|---------------------|---------------------|----------|
| **docker-deploy** | Yes | Copies automatically | 5 min | Yes | Yes (warning only) | Low | Docker + SSH | No | Single VPS, compose-first projects |
| Terraform | No | Via secrets provider | 30–60 min | No | Depends on provider | High | Provider-specific | No | Infrastructure provisioning |
| Ansible | Partial (via module) | Via Vault or file copy | 30–60 min | No | Yes | Medium | Python on remote | No | Multi-server configuration management |
| Docker remote context | Yes | Must exist on remote | 10 min | Yes | Yes | Low | Docker daemon socket open | No | Remote development workflows |
| Manual SSH scripts | Yes (manual) | Manual copy | Varies | Partial | Depends on script | Low (but fragile) | Docker + SSH | No | Ad-hoc one-off deploys |
| docker-compose + Watchtower | Yes | Must be pre-placed | 30 min | Yes | Yes | Medium | Docker + Watchtower container | No | Continuous auto-update from registry |
| Portainer | Yes | Via UI | 15 min | Yes | Yes | Low (UI-driven) | Docker + Portainer agent | No | Teams needing a UI dashboard |
| Kamal | No (kamal format) | Via .env.production file | 20–30 min | No | Yes | Medium | Docker + Traefik | No | Rails / single-container Ruby apps |
| Full CI/CD (GitHub Actions / GitLab CI) | Yes (via scripts) | Via CI secrets | 1–2 hours | No | Depends on setup | High | Docker + SSH agent | Yes (usually) | Team projects with automated pipelines |

**Notes on table values:**
- "Compose-centric design" means the tool is built around docker-compose as a first-class primitive.
- "SSH best practices (no root required)" means day-to-day deploys run as a non-root user; docker-deploy uses root only during `--init` for initial user setup.
- "Remote dependencies" describes what must be installed and running on the VPS, beyond the OS itself.
- Docker remote context requires project files to already exist on the remote, so it does not solve the local-to-remote file copy problem.

---

## When to use docker-deploy

- You are a solo developer or small team with a single VPS
- Your project is docker-compose-native (compose.yaml or docker-compose.yml is the source of truth)
- You want a single command (`docker deploy`) with no CI/CD infrastructure to maintain
- You need `.env` and secrets copied alongside compose files automatically
- You want SSH key authentication with strict knownhosts verification
- You do not want — or cannot install — git on the VPS

---

## When NOT to use docker-deploy

- **Multi-server orchestration:** Use Ansible (config management) or Terraform (infrastructure provisioning) when you need to coordinate multiple hosts.
- **Container registry + rolling deploys:** Use Kamal or a CI/CD pipeline if you are building images in CI, pushing to a registry, and need zero-downtime rolling updates.
- **Team with UI management needs:** Use Portainer if your team needs a web dashboard for container monitoring, log browsing, and manual operations without SSH.
- **Infrastructure-as-code requirements:** Use Terraform if your organisation requires declarative state management, drift detection, and audit trails for infrastructure.
- **Automated pipeline with secrets management:** Use GitHub Actions or GitLab CI if you already have a CI system and want environment-specific secrets injected from a vault rather than copied from a local file.
