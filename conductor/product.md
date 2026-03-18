# Initial Concept
A sophisticated AI-powered command-line coding assistant built with Go using hexagonal (clean) architecture principles. The agent provides an interactive chat interface for code exploration, editing, and analysis with integrated tool capabilities and advanced AI features.

# Product Vision
To empower developers and SREs with a highly capable, autonomous, and safe AI partner that lives in the terminal, understands complex codebases, and—most importantly—automates the heavy lifting of SRE workflows and complex troubleshooting. This agent is designed to bridge the gap between high-level reasoning (Claude) and the frontline reality of maintaining reliable infrastructure.

# Target Users
- **SRE & DevOps Engineers:** The primary audience, using the AI investigation server to automate root cause analysis for infrastructure incidents and maintain system reliability across multi-cloud environments.
- **Senior Software Engineers:** Seeking an efficient pair-programmer for complex refactorings and troubleshooting application-level bugs.
- **Go Developers:** Working on projects where clean architecture and type safety are paramount.

# Core Goals
1. **Automated Reliability (SRE Focus):** Drastically reduce Mean Time To Resolution (MTTR) by leveraging AI for automated, intelligent troubleshooting and root cause analysis.
2. **Autonomous Troubleshooting:** Enable the agent to independently investigate alerts, gather logs, query metrics, and propose fixes with minimal human intervention.
3. **Architectural Integrity:** Demonstrate the power of hexagonal architecture in building extensible and testable AI systems that can adapt to various monitoring and alerting backends.
4. **Safety and Control:** Provide robust sandboxing and validation for AI-driven shell commands and file edits, ensuring automated fixes don't make the incident worse.

# Key Features
- **AI Investigation Server:** Automated, real-time investigation of alerts from Prometheus, GCP Monitoring, and other sources with automated findings and escalation logic.
- **Root Cause Analysis (RCA):** Intelligent correlation of alerts, logs, and metrics to identify the definitive cause of system failures.
- **Interactive Chat Interface:** Natural language interaction for ad-hoc troubleshooting and system exploration.
- **Hexagonal Architecture:** Modular design with clear separation between domain logic and infrastructure adapters.
- **Subagent Delegation:** Specialized agents for specific SRE tasks (log analysis, metric querying, documentation).
- **Plan Mode:** Review and approve AI-proposed troubleshooting steps or fixes before they are applied.
- **Integrated Toolbelt:** Safe file manipulation, shell access, and web fetching for diagnostic purposes.