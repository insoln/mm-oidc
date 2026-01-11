# Mattermost OIDC Plugin – Agent Skills

This directory contains [Agent Skills](https://agentskills.io/) for the Mattermost OIDC plugin, following the [VS Code Copilot Agent Skills specification](https://code.visualstudio.com/docs/copilot/customization/agent-skills). These skills provide specialized knowledge and workflow automation for GitHub Copilot and other AI coding assistants.

## What are Agent Skills?

Agent Skills are portable, reusable knowledge packages that help AI agents understand specialized workflows, best practices, and domain-specific tasks. They consist of markdown files with YAML frontmatter that describe when and how to perform specific operations.

## Available Skills

### Development & Build (4 skills)

1. **[dev-environment](dev-environment/)** - Local development stack setup and management
   - Start/stop Docker Compose stack (Mattermost + Keycloak + Postgres + proxy)
   - View logs and troubleshoot local environment
   - Bootstrap Keycloak and configure plugin automatically

2. **[plugin-build](plugin-build/)** - Build server, webapp, and package plugin
   - Compile Go backend and React frontend
   - Create plugin archive for distribution
   - Incremental builds for development

3. **[e2e-testing](e2e-testing/)** - End-to-end testing with Playwright
   - Write and run browser-based tests
   - Debug test failures
   - Integrate tests into CI/CD pipelines

4. **[release-management](release-management/)** - Prepare and publish releases
   - Version management and semantic versioning
   - Build release artifacts
   - Publish to GitHub Releases

### Configuration & Deployment (4 skills)

5. **[keycloak-setup](keycloak-setup/)** - Configure Keycloak as OIDC provider
   - Create realm and confidential client
   - Add protocol mappers for claims
   - Set up role mapping for admin promotion

6. **[plugin-install](plugin-install/)** - Install and configure plugin in Mattermost
   - Upload plugin archive
   - Configure OIDC settings (issuer, client ID/secret, scopes)
   - Validate installation and test login flow

7. **[proxy-setup](proxy-setup/)** - Configure reverse proxy for automatic OIDC redirect
   - Deploy Nginx proxy with cookie detection
   - Configure redirect rules based on MMAUTHTOKEN
   - Integrate with Kubernetes/Docker deployments

8. **[config-migration](config-migration/)** - Migrate configuration between versions/environments
   - Backup and restore configuration
   - Handle version upgrades
   - Migrate between dev/staging/production

### Testing & Validation (3 skills)

9. **[oidc-flow-test](oidc-flow-test/)** - Test OIDC authentication flow
   - Manual testing procedures
   - Automated Playwright tests
   - Debug authentication issues

10. **[health-check](health-check/)** - Verify system health
    - Check plugin, Mattermost, Keycloak health
    - Set up monitoring and alerts
    - Validate deployments

11. **[security-audit](security-audit/)** - Security review and best practices
    - HTTPS enforcement and certificate validation
    - Secret management and rotation
    - PKCE, state, and nonce validation
    - Vulnerability scanning

### Operations & Support (3 skills)

12. **[troubleshooting](troubleshooting/)** - Diagnose and resolve issues
    - Authentication failures
    - Configuration errors
    - Redirect loops
    - Performance problems

13. **[user-provisioning](user-provisioning/)** - User management and role mapping
    - Configure automatic user provisioning
    - Map OIDC claims to Mattermost fields
    - Set up role-based access control
    - Migrate existing users from password to OIDC

14. **[observability](observability/)** - Logging, metrics, and tracing
    - Structured logging with correlation IDs
    - Prometheus metrics export
    - OpenTelemetry distributed tracing
    - Monitoring dashboards and alerts

## Using Agent Skills

### In VS Code with GitHub Copilot

Agent Skills are automatically detected by GitHub Copilot when they're in `.github/skills/` directory. Simply mention tasks related to the skills in your prompts:

**Examples:**
```
"Set up the local dev environment"
→ Copilot will reference dev-environment skill

"How do I configure Keycloak for this plugin?"
→ Copilot will reference keycloak-setup skill

"Run end-to-end tests"
→ Copilot will reference e2e-testing skill
```

### Viewing Skills

Each skill directory contains a `SKILL.md` file with:
- **YAML frontmatter**: Metadata (name, description, tags)
- **Summary**: Overview of what the skill does
- **When to Use**: Scenarios where the skill applies
- **Detailed Instructions**: Step-by-step procedures, commands, examples
- **Related Skills**: Cross-references to other relevant skills
- **References**: Links to documentation and resources

### Skill Discovery

Browse skills by category:

```bash
# List all skills
ls -1 .github/skills/

# Search for skills by tag
grep -r "tags:" .github/skills/*/SKILL.md

# Find skills related to testing
grep -l "testing" .github/skills/*/SKILL.md
```

## Skill Development

### Creating a New Skill

1. Create skill directory:
   ```bash
   mkdir -p .github/skills/my-skill
   ```

2. Create `SKILL.md` with frontmatter:
   ```markdown
   ---
   name: my-skill
   description: Brief description of what this skill does
   license: Apache-2.0
   metadata:
     author: insoln
     version: "1.0"
     category: development
     tags: [tag1, tag2]
   ---
   
   # My Skill
   
   ## Summary
   ...
   ```

3. Add detailed content:
   - When to use this skill
   - Prerequisites
   - Step-by-step instructions
   - Examples and commands
   - Troubleshooting
   - Related skills

### Best Practices

1. **Clear naming**: Use kebab-case, descriptive names
2. **Comprehensive descriptions**: Help AI agents know when to use the skill
3. **Concrete examples**: Include actual commands and code snippets
4. **Cross-references**: Link to related skills and documentation
5. **Regular updates**: Keep skills in sync with code and documentation changes

## Architecture

### Skill Organization

```
.github/skills/
├── README.md                    # This file
├── dev-environment/
│   └── SKILL.md
├── keycloak-setup/
│   └── SKILL.md
├── plugin-build/
│   └── SKILL.md
...
```

### Skill Metadata

Each skill includes:
- **name**: Unique identifier (matches directory name)
- **description**: Clear description for AI agent discovery
- **license**: Open source license (Apache-2.0)
- **metadata**: Additional info (author, version, category, tags)

### Categories

- **development**: Local dev, building, testing
- **configuration**: Setup, configuration management
- **deployment**: Installation, production deployment
- **testing**: Test execution, validation
- **operations**: Monitoring, maintenance, support
- **security**: Security audits, compliance

## Integration

### GitHub Copilot

Skills are automatically available to GitHub Copilot in VS Code when this repository is open. Copilot uses skills to provide context-aware suggestions and automate workflows.

### Custom Agents

Skills can be consumed by custom AI agents following the AgentSkills.io specification. The standard YAML frontmatter and Markdown format makes them portable across different agent implementations.

### CI/CD

Skills document workflows that can be automated in CI/CD pipelines. Many skills include example GitHub Actions configurations.

## Validation

### Skill Spec Compliance

Skills follow the [AgentSkills.io specification](https://agentskills.io/specification):
- ✅ YAML frontmatter with required fields (name, description)
- ✅ Markdown body with structured content
- ✅ Clear use cases and prerequisites
- ✅ Step-by-step instructions with examples
- ✅ Cross-references to related skills

### Testing Skills

While skills are documentation, their effectiveness can be validated:
1. **Manual testing**: Follow instructions in each skill
2. **AI agent testing**: Prompt GitHub Copilot with skill-related tasks
3. **Documentation accuracy**: Ensure commands and examples work
4. **Cross-reference validity**: Verify linked skills exist

## Contributing

When adding new features or workflows to the plugin:

1. **Document in skills**: Create or update relevant skills
2. **Cross-reference**: Link skills to main documentation
3. **Test instructions**: Verify all commands and examples work
4. **Update skill list**: Add new skills to this README

## References

- [VS Code Copilot Agent Skills Documentation](https://code.visualstudio.com/docs/copilot/customization/agent-skills)
- [AgentSkills.io Specification](https://agentskills.io/specification)
- [GitHub Docs: About Agent Skills](https://docs.github.com/en/copilot/concepts/agents/about-agent-skills)
- [Mattermost OIDC Plugin Documentation](../../docs/)

## License

Agent Skills in this repository are licensed under Apache-2.0, consistent with the main plugin license.
