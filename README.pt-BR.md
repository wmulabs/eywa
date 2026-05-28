<p align="center">
  <img src="docs/assets/banner.png" alt="Eywa — Orquestração de IA orientada a eventos para Go" width="100%"/>
</p>

<h1 align="center">🌿 Eywa</h1>

<p align="center">
  <em>Orquestração de IA multi-agente, orientada a eventos, para Go</em>
</p>

<p align="center">
  🌐 <strong>Português</strong> · <a href="README.md">English</a>
</p>

<p align="center">
  <a href="https://pkg.go.dev/github.com/wmulabs/eywa"><img src="https://pkg.go.dev/badge/github.com/wmulabs/eywa.svg" alt="Go Reference"/></a>
  <img src="https://img.shields.io/badge/Go-1.25+-00ADD8?logo=go&logoColor=white" alt="Go version"/>
  <img src="https://img.shields.io/badge/License-Apache--2.0-blue.svg" alt="License"/>
  <img src="https://img.shields.io/badge/Architecture-Hexagonal-6c3483" alt="Hexagonal"/>
  <img src="https://img.shields.io/badge/LLMs-8+_provedores-FF6B35" alt="LLM Providers"/>
  <a href="https://github.com/wmulabs/eywa/actions/workflows/ci.yml"><img src="https://github.com/wmulabs/eywa/actions/workflows/ci.yml/badge.svg" alt="CI"/></a>
  <a href="https://codecov.io/gh/wmulabs/eywa"><img src="https://codecov.io/gh/wmulabs/eywa/graph/badge.svg" alt="Coverage"/></a>
</p>

<p align="center">
  <a href="https://github.com/sponsors/wmulabs"><img src="https://img.shields.io/badge/Sponsor-GitHub-%23EA4AAA?logo=github" alt="GitHub Sponsors"/></a>
  <a href="https://buymeacoffee.com/wmulabs"><img src="https://img.shields.io/badge/Buy%20Me%20a%20Coffee-wmulabs-%23FFDD00?logo=buymeacoffee&logoColor=black" alt="Buy Me a Coffee"/></a>
</p>

---

> *Na crença Na'vi, Eywa é a consciência de Pandora — uma vasta rede viva que conecta todas as criaturas, carrega a memória dos ancestrais e orquestra o equilíbrio do mundo. Quando um ser envia um sinal para o Weave, Eywa encontra o Spirit com a sabedoria para responder.*
>
> *É exatamente isso que esta biblioteca faz pelos seus sistemas de IA.*

---

## O problema com "IA em produção"

Você adicionou uma chamada LLM a um webhook handler. Funciona. Então seus usuários começam a enviar mensagens concorrentes e você obtém respostas duplicadas. Você adiciona lock no Redis. Então as conversas crescem além da janela de contexto. Você adiciona um summarizer. Então você precisa que a IA tome ações reais. Você adiciona tool use. Então você precisa de supervisão humana. Você adiciona…

**Você não está mais construindo um produto. Está construindo infraestrutura.**

Eywa é essa infraestrutura — um framework Go battle-tested que gerencia o ciclo de vida completo de uma interação de IA: receber eventos de qualquer canal, enriquecê-los com contexto, rotear para o Spirit correto, executar tool calls, manter memória, observar tudo e entregar respostas. Todas as partes difíceis, feitas uma vez, feitas certo.

**Pare de colar chamadas LLM em webhook handlers. Comece a orquestrar.**

---

## 🌍 A Mitologia

Cada nome no Eywa carrega significado do mundo de Pandora. Não é cosmético — os nomes codificam a arquitetura:

| 🔮 Termo | O que é |
|---------|---------|
| **Weave** | A rede viva — o motor de runtime que conecta tudo |
| **Spirit** | Um agente de IA: nomeado, com system prompt, config de modelo e ações permitidas |
| **Pulse** | Um sinal entrando no Weave — uma mensagem, um webhook, um gatilho |
| **Oracle** | Um provedor de LLM — a fonte de sabedoria (Anthropic, OpenAI, Gemini e mais) |
| **Action** | Uma ferramenta que o Oracle pode invocar — uma capacidade real que o Spirit exerce |
| **Scout** | Enriquece um Pulse com contexto antes que o Spirit o veja |
| **Pathfinder** | Roteia um Pulse para o Spirit correto quando múltiplos estão disponíveis |
| **Voice** | O canal pelo qual a resposta do Spirit chega ao mundo |
| **Memory** | Estado efêmero de conversa — a memória de trabalho do Spirit por usuário |
| **Echo** | Histórico persistido de mensagens — o registro permanente |
| **Chronicle** | Log de auditoria de cada interação — para observabilidade |
| **Bond** | Lock distribuído — previne race conditions entre Pulses concorrentes |
| **Keeper** | Backend de agendamento (ex: Cloud Tasks) — guarda eventos futuros |
| **Ritual** | Um evento agendado ou recorrente — uma cerimônia que o Keeper realiza |
| **Archivist** | Sumariza conversas longas para o Oracle nunca perder o contexto |
| **Receptor** | Converte payloads brutos de webhook em Pulses |
| **Link** | Conecta um tipo de evento a Scouts, um Pathfinder e Spirits permitidos |
| **Vault** | Armazenamento de objetos para arquivos de mídia (ex: GCS) |
| **Lens** | Processador de mídia — transcreve áudio, analisa imagens, extrai documentos |
| **Lore** | A base de conhecimento — documentos que o Spirit pode pesquisar em runtime (RAG) |
| **Imprint** | Memória de longo prazo do usuário — fatos que persistem entre todas as conversas |
| **Ledger** | Rastreamento de uso de tokens e custo — com orçamentos e roteamento inteligente |
| **Vigil** | Takeover humano — um operador assumindo uma sessão de conversa ao vivo |
| **Rite** | Fluxo de aprovação assíncrono — o Spirit aguarda uma decisão humana antes de agir |
| **Conduit** | Cliente MCP — conecta a servidores de ferramentas externos via Model Context Protocol |

---

## ✨ O que você constrói com Eywa

🤖 **Agentes de IA conversacionais** que gerenciam milhares de usuários simultâneos, mantêm memória entre sessões, chamam APIs externas e nunca produzem respostas duplicadas.

🧭 **Pipelines multi-agente** onde Pulses são roteados entre Spirits especializados — um orquestrador delega a um pesquisador, que delega a um escritor — com profundidade configurável e execução paralela.

📚 **Assistentes com RAG** que pesquisam uma base de conhecimento privada (Lore) no momento da consulta, recuperando os chunks relevantes e injetando-os como contexto antes do Oracle raciocinar.

🧠 **Experiências personalizadas** onde um Spirit lembra preferências do usuário, objetivos passados e fatos (Imprint) — não apenas desta sessão, mas de todas as conversas que o usuário já teve.

🙋 **Fluxos com humano no loop** onde ações críticas pausam para aprovação do operador (Rite) e operadores podem assumir conversas ao vivo diretamente (Vigil) — e depois devolver ao AI.

🔧 **Tool use nativo MCP** onde Spirits chamam ferramentas de qualquer servidor Model Context Protocol (Conduit), descobrindo capacidades em runtime e chamando-as como Actions nativas.

⚡ **Processamento assíncrono de eventos** onde webhooks retornam em milissegundos e o processamento acontece de forma confiável em segundo plano via Cloud Tasks, com retry automático e deduplicação.

---

## 🔄 O Pipeline

Cada Pulse percorre o mesmo pipeline:

```
Pulse → [Guard] → [Lock] → [Scouts] → [Pathfinder] → Spirit → Oracle → Actions → Voice
```

| Etapa | Descrição |
|-------|-----------|
| 🛡️ **Guard** | Bloqueia ou permite o Pulse com base em regras de allow/block |
| 🔒 **Lock** | Adquire um Bond — apenas um Pulse por usuário por vez |
| 🔭 **Scouts** | Executam sequencialmente, enriquecendo o Pulse com conhecimento |
| 🧭 **Pathfinder** | Seleciona o Spirit correto do conjunto permitido |
| 👻 **Spirit** | Fornece system prompt, config de modelo e Actions permitidas |
| 🔮 **Oracle** | Raciocina sobre memória + Lore, chama Actions em loop até concluir |
| ⚡ **Actions** | Executam tool calls (buscar dados, enviar mensagens, atualizar registros) |
| 📢 **Voice** | Entrega a resposta final pelo canal apropriado |

O pipeline também gerencia configuração de memória, coalescência de mensagens, arquivamento de conversas, verificações de takeover humano e persistência completa — tudo transparente para o código da sua aplicação.

---

## 📦 Instalação

```bash
go get github.com/wmulabs/eywa
```

Sub-módulos são opt-in — inclua apenas o que precisar:

```bash
# Adaptadores de infraestrutura
go get github.com/wmulabs/eywa/mongo              # MongoDB: Spirits, Echoes, Chronicles, Rituals, Lore, Rites
go get github.com/wmulabs/eywa/redis              # Redis: Memory, Bond, Vigil, rate limiter

# Provedores de LLM
go get github.com/wmulabs/eywa/providers/anthropic # Anthropic Claude (Sonnet, Haiku, Opus)
go get github.com/wmulabs/eywa/providers/openai   # OpenAI GPT — e qualquer API compatível com OpenAI
go get github.com/wmulabs/eywa/providers/gemini   # Google Gemini
go get github.com/wmulabs/eywa/providers/bedrock  # AWS Bedrock (Converse API — qualquer modelo Bedrock)
go get github.com/wmulabs/eywa/providers/vertexai # Google Vertex AI (autenticação ADC, sem API key)

# API REST de gerenciamento
go get github.com/wmulabs/eywa/fiber              # Fiber: API REST completa de gerenciamento

# Servidores de ferramentas externos
go get github.com/wmulabs/eywa/mcp               # MCP Conduit: conecta a qualquer servidor MCP

# Canais
go get github.com/wmulabs/eywa/channels/whatsapp  # WhatsApp via 360Dialog / Twilio

# Integrações GCP
go get github.com/wmulabs/eywa/gcp/cloudtasks     # Cloud Tasks: dispatch assíncrono + Rituals
go get github.com/wmulabs/eywa/gcp/gcs            # GCS Vault para armazenamento de mídia
go get github.com/wmulabs/eywa/gcp/gemini         # Gemini: processamento de imagem/áudio/documento
```

---

## 🚀 Quick Start

```go
package main

import (
    "context"
    "fmt"
    "os"

    eywa "github.com/wmulabs/eywa"
    eywamongo "github.com/wmulabs/eywa/mongo"
    eywaredis "github.com/wmulabs/eywa/redis"
    eywaopenai "github.com/wmulabs/eywa/providers/openai"
)

func main() {
    ctx := context.Background()

    mongoConn, err := eywamongo.NewMongoConnection(ctx, os.Getenv("MONGO_URL"), "mydb", "myapp")
    if err != nil {
        log.Fatalf("failed to connect to MongoDB: %v", err)
    }
    defer mongoConn.DisconnectMongoDB(ctx)

    redisConn, err := eywaredis.NewRedisConnection(ctx, os.Getenv("REDIS_URL"), "myapp")
    if err != nil {
        log.Fatalf("failed to connect to Redis: %v", err)
    }
    defer redisConn.DisconnectRedisDB(ctx)

    db := mongoConn.GetDatabase()

    weave, err := eywa.NewWeaveBuilder(ctx).
        WithRepositories(
            eywamongo.NewSpiritRepository(db),
            eywaredis.NewMemoryRepository(redisConn.GetClient(), "myapp", "prod", 3600, nil),
            eywamongo.NewEchoRepository(db),
            eywamongo.NewChronicleRepository(db),
        ).
        WithBond(eywaredis.NewBondManager(redisConn.GetClient())).
        WithActionRegistry(eywa.NewActionRegistry()).
        WithScoutRegistry(eywa.NewScoutRegistry()).
        AddOracle(eywaopenai.NewOracle(os.Getenv("OPENAI_API_KEY"))).
        WithConfig(eywa.DefaultWeaveConfig()).
        Build()
    if err != nil {
        panic(err)
    }

    weave.RegisterEventConfiguration(
        eywa.NewLink("customer_message").
            WithDefaultSpirit("support_spirit").
            Build(),
    )

    pulse := eywa.NewPulse(eywa.MemoryKey{Channel: "api", User: "user_123"}).
        WithUserMessage("Qual o status do meu pedido #4821?").
        Build()

    result, _ := weave.ProcessEventByKey(ctx, "customer_message", pulse)
    fmt.Println(result.Message)
}
```

---

## 👻 Definindo Spirits

Spirits são os agentes do seu sistema. Cada um tem um nome, uma personalidade (system prompt), uma configuração de modelo e uma lista de Actions que pode chamar.

```go
spirit := &eywa.Spirit{
    Name:         "support_agent",
    Description:  "Especialista em suporte ao cliente",
    SystemPrompt: `Você é um agente de suporte da Acme Corp.
Tem acesso a rastreamento de pedidos e ferramentas de reembolso.
Seja sempre conciso e profissional.`,
    AllowedActions: []eywa.AllowedAction{
        {Name: "track_order"},
        {Name: "request_refund"},
    },
    ModelConfig: eywa.SpiritModel{
        Provider:    "openai",
        Model:       "gpt-4o-mini",
        Temperature: 0.5,
        MaxTokens:   1000,
    },
    IsActive:  true,
    CreatedAt: time.Now(),
}
spiritRepo.Create(ctx, spirit)
```

> **Dica:** Spirits são versionados. Cada chamada `Update` cria uma nova versão. Faça rollback com `POST /api/v1/spirits/:name/activate` + `{"version": N}`.

---

## ⚡ Actions Customizadas (Tool Use)

Dê capacidades reais aos Spirits implementando a interface `Action`:

```go
type TrackOrderAction struct {
    orderService *OrderService
}

func (a *TrackOrderAction) GetName() string        { return "track_order" }
func (a *TrackOrderAction) GetDescription() string { return "Rastreia um pedido pelo ID." }
func (a *TrackOrderAction) IsCritical() bool       { return false }

func (a *TrackOrderAction) GetParameters() map[string]interface{} {
    return map[string]interface{}{
        "type": "object",
        "properties": map[string]interface{}{
            "order_id": map[string]interface{}{
                "type":        "string",
                "description": "O ID do pedido a rastrear",
            },
        },
        "required": []string{"order_id"},
    }
}

func (a *TrackOrderAction) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
    status, err := a.orderService.GetStatus(ctx, args["order_id"].(string))
    if err != nil {
        return "", eywa.NewInfrastructureError("falha ao buscar pedido", err)
    }
    return fmt.Sprintf("Status do pedido: %s", status), nil
}
```

**Actions nativas:**

| Construtor | Nome da ferramenta | Descrição |
|-----------|-------------------|-----------|
| `eywa.NewScheduleRitualAction()` | `schedule_ritual` | Agenda um Pulse futuro via Keeper |
| `eywa.NewListRitualsAction()` | `list_rituals` | Lista Rituals pendentes para o usuário atual |
| `eywa.NewCancelRitualAction()` | `cancel_ritual` | Cancela um Ritual pendente |
| `eywa.NewUpdateSubjectAction()` | `update_subject` | Rastreia um subject key e acumula fatos na Memory |
| `eywa.NewRememberFactAction()` | `remember_fact` | Armazena um fato persistente do usuário no Imprint |
| `eywa.NewForgetFactAction()` | `forget_fact` | Remove um fato do Imprint |
| `eywa.NewSearchLoreAction()` | `search_lore` | Pesquisa a base de conhecimento (RAG) |
| `eywa.NewRequestRiteAction()` | `request_rite` | Solicita aprovação humana antes de prosseguir |

---

## 🔭 Enriquecimento de Contexto com Scouts

Scouts executam antes da seleção do Spirit e injetam conhecimento no Pulse. São o lugar certo para carregar dados do usuário, feature flags ou contexto externo.

```go
type UserProfileScout struct{ repo *UserRepository }

func (s *UserProfileScout) GetName() string { return "user_profile" }

func (s *UserProfileScout) IsApplicable(pulse *eywa.Pulse) bool {
    return pulse.ContactPhone != ""
}

func (s *UserProfileScout) Harvest(ctx context.Context, pulse *eywa.Pulse) error {
    user, err := s.repo.FindByPhone(ctx, pulse.ContactPhone)
    if err != nil {
        return nil // não fatal — Pulse continua sem esse dado
    }
    pulse.Knowledge["user_name"]   = user.Name
    pulse.Knowledge["user_tier"]   = user.Tier
    pulse.Knowledge["open_orders"] = user.OpenOrderCount
    return nil
}
```

> Scouts executam **sequencialmente** dentro do `ScoutTimeout` (padrão: 15s). Um erro de Scout é logado mas nunca aborta o pipeline.

---

## 🧭 Roteamento Multi-Agente com Pathfinders

Roteia Pulses para o Spirit correto automaticamente com base no conteúdo da mensagem:

```go
// Roteamento via LLM — um modelo barato classifica a intenção
weave, _ := eywa.NewWeaveBuilder(ctx).
    WithDefaultLLMPathfinder("openai", "gpt-4o-mini", 0.1).
    Build()

weave.RegisterEventConfiguration(
    eywa.NewLink("customer_message").
        WithSpirits("support_agent", "sales_agent", "billing_agent").
        WithDefaultSpirit("support_agent").
        Build(),
)
```

---

## 🤝 Orquestração Multi-Agente

Construa pipelines de Spirits especializados. Um Orquestrador delega subtarefas para Sub-Spirits via a ferramenta `summon_spirit` nativa.

```go
coordinator := &eywa.Spirit{
    Name:        "coordinator",
    Description: "Orquestra pesquisa e redação",
    SystemPrompt: `Você coordena agentes especialistas.
Para cada solicitação: acione o pesquisador, depois passe os resultados para o escritor.`,
    Type: eywa.SpiritTypeOrchestrator,
    OrchestratorConfig: eywa.OrchestratorConfig{
        SubSpirits:     []string{"researcher", "writer"},
        MaxDepth:       2,
        ParallelSummon: false,
    },
    ModelConfig: eywa.SpiritModel{Provider: "openai", Model: "gpt-4o-mini"},
    IsActive: true,
}
```

---

## 📚 RAG com Lore

Dê aos Spirits uma base de conhecimento pesquisável. Ingira documentos e deixe o Oracle recuperar chunks relevantes em tempo de consulta.

```go
// Ingira documentos
loreRepo := eywamongo.NewLoreRepository(db)
lore := &eywa.Lore{
    Name:        "product_docs",
    Description: "Documentação e FAQs do produto",
    Chunks: []eywa.LoreChunk{
        {Content: "A política de devolução permite retornos em 30 dias..."},
        {Content: "Para rastrear seu pedido, acesse meusite.com/rastrear com seu ID..."},
    },
}
loreRepo.Create(ctx, lore)

// Conecta ao Weave
weave, _ := eywa.NewWeaveBuilder(ctx).
    WithLoreRepository(loreRepo).
    WithLoreEmbedder(myEmbedder). // implementa LoreEmbedder
    Build()

// Spirit agora pode chamar search_lore
spirit.AllowedActions = []eywa.AllowedAction{{Name: "search_lore"}}
```

> **Nota de arquitetura:** `LoreEmbedder` e `LoreStore` são ports. O adaptador MongoDB usa busca full-text por padrão. Troque por pgvector, Qdrant, Pinecone ou Weaviate para busca vetorial semântica.

---

## 🧠 Memória de Longo Prazo com Imprint

Spirits lembram preferências e fatos do usuário entre sessões — não apenas dentro de uma conversa.

```go
weave, _ := eywa.NewWeaveBuilder(ctx).
    WithImprintRepository(eywamongo.NewImprintRepository(db)).
    WithImprintExtraction(eywa.ImprintExtractionConfig{
        Enabled:    true,
        MaxFacts:   50,
        Categories: []string{"preference", "personal", "goal"},
    }).
    Build()
```

Quando `ImprintExtractionConfig.Enabled` é true, o engine extrai e armazena fatos das mensagens do usuário automaticamente. Spirits também têm as Actions explícitas `remember_fact` e `forget_fact`.

Um usuário que diz "prefiro tom formal" em uma conversa terá essa preferência injetada como contexto na próxima — sem precisar repetir.

---

## 🙋 Humano no Loop

### Vigil — Takeover de Operador

Um operador pode assumir exclusivamente qualquer conversa ao vivo. Enquanto o assento está ocupado, a IA é bloqueada e o operador lida com as mensagens diretamente. O assento tem TTL — expira automaticamente se o operador ficar em silêncio.

```go
vigilRepo := eywaredis.NewVigilRepository(client, "myapp", "prod")

weave, _ := eywa.NewWeaveBuilder(ctx).
    WithVigilRepository(vigilRepo).
    WithVigilConfig(eywa.VigilConfig{InactivityTimeout: 30 * time.Minute}).
    Build()
```

Quando um assento Vigil está ativo, `ProcessEventByKey` retorna `ErrSessionHeld`. A API de gerenciamento expõe:

```
GET    /api/v1/vigil                       # listar todos os assentos ativos
POST   /api/v1/vigil/:memoryKey            # operador assume o assento
POST   /api/v1/vigil/:memoryKey/echoes     # operador envia mensagem diretamente
DELETE /api/v1/vigil/:memoryKey            # liberar — IA retoma
GET    /api/v1/vigil/:memoryKey            # status do assento
```

Assine `GET /api/v1/sse/vigil` para eventos em tempo real de `vigil_acquired` / `vigil_released` em todas as sessões.

### Rite — Fluxo de Aprovação

Um Spirit pode pausar e solicitar aprovação humana antes de executar uma ação crítica. O Rite é armazenado no MongoDB e o operador aprova ou rejeita via API.

```go
// Spirit chama a action nativa request_rite
spirit.AllowedActions = []eywa.AllowedAction{{Name: "request_rite"}}

weave, _ := eywa.NewWeaveBuilder(ctx).
    WithRiteRepository(eywamongo.NewRiteRepository(db)).
    Build()
```

```
GET  /api/v1/rites              # listar rites pendentes
POST /api/v1/rites/:id/approve  # aprovar — Spirit retoma execução
POST /api/v1/rites/:id/reject   # rejeitar — Spirit recebe a decisão e responde
```

Assine `GET /api/v1/sse/rites` para eventos em tempo real de `rite_created` / `rite_decided` / `rite_expired`.

---

## 🔌 Integração MCP (Conduit)

Conecte Spirits a qualquer servidor Model Context Protocol. Ferramentas são descobertas automaticamente na inicialização e registradas como Actions com o prefixo `<conduit_name>__<tool_name>`.

```go
import eywamcp "github.com/wmulabs/eywa/mcp"

conduit := eywamcp.NewConduit(eywamcp.ConduitConfig{
    Name:      "my_tools",
    Transport: "http",
    URL:       "http://localhost:3001",
    Timeout:   15 * time.Second,
})

weave, _ := eywa.NewWeaveBuilder(ctx).
    WithConduit(conduit). // ferramentas registradas automaticamente no Build()
    Build()

// Spirit referencia ferramentas MCP pelo nome com prefixo
spirit.AllowedActions = []eywa.AllowedAction{
    {Name: "my_tools__search"},
    {Name: "my_tools__create_task"},
}
```

---

## 💰 Rastreamento de Custo com Ledger

Rastreie uso de tokens por Spirit com orçamentos mensais e roteamento automático de modelos.

```go
ledgerRepo := eywamongo.NewLedgerRepository(db)

ledgerRepo.SetBudget(ctx, eywa.TokenBudget{
    SpiritID:          "assistant",
    MonthlyTokenLimit: 100_000,
    OnExceed:          "downgrade", // "block" | "downgrade" | "alert"
    DowngradeModel:    eywa.SpiritModel{Provider: "openai", Model: "gpt-4o-mini"},
    AlertThreshold:    0.8,
})

weave, _ := eywa.NewWeaveBuilder(ctx).
    WithLedgerRepository(ledgerRepo).
    Build()
```

---

## 🔮 Provedores de LLM

Eywa suporta 8+ provedores de LLM nativamente. Misture-os livremente — modelos diferentes para raciocínio, roteamento e arquivamento.

### Provedores nativos

| Provedor | Pacote | Construtor |
|----------|--------|------------|
| 🟣 Anthropic Claude | `providers/anthropic` | `anthropic.NewOracle(apiKey)` |
| 🟢 OpenAI GPT | `providers/openai` | `openai.NewOracle(apiKey)` |
| 🔵 Google Gemini | `providers/gemini` | `gemini.NewOracle(ctx, apiKey)` |
| 🟠 AWS Bedrock | `providers/bedrock` | `bedrock.NewOracle(ctx, region)` |
| 🔴 Google Vertex AI | `providers/vertexai` | `vertexai.NewOracle(ctx, project, location)` |

### Provedores compatíveis com OpenAI (via `providers/openai`)

Qualquer serviço que fale o formato da API OpenAI funciona como Oracle:

```go
import "github.com/wmulabs/eywa/providers/openai"

// Modelos locais
ollama   := openai.NewOllamaOracle("http://localhost:11434")

// Provedores cloud
groq     := openai.NewGroqOracle(os.Getenv("GROQ_API_KEY"))
mistral  := openai.NewMistralOracle(os.Getenv("MISTRAL_API_KEY"))
together := openai.NewTogetherOracle(os.Getenv("TOGETHER_API_KEY"))
router   := openai.NewOpenRouterOracle(os.Getenv("OPENROUTER_API_KEY"))
xai      := openai.NewXAIOracle(os.Getenv("XAI_API_KEY"))
```

---

## 🗓️ Memória de Conversa e Arquivamento

Memória é automática. Para conversas longas, ative o Archivist para evitar overflow de contexto:

```go
weave, _ := eywa.NewWeaveBuilder(ctx).
    WithDefaultLLMArchivist("anthropic", "claude-haiku-4-5-20251001", 20).
    WithArchivistConfig(0.1, 512).
    Build()
```

Quando a conversa atinge 20 mensagens, o Archivist sumariza a metade mais antiga e armazena na Memory. O Oracle recebe o resumo — o fio da conversa nunca se rompe.

---

## 🌐 API REST de Gerenciamento (Fiber)

Monte a API completa de gerenciamento em duas linhas:

```go
import eywafiber "github.com/wmulabs/eywa/fiber"

app := fiber.New()
eywafiber.RegisterManagementRoutes(app, weave, eywafiber.ManagementDeps{
    APIKeys:            map[string]string{"my-api-key": "admin"},
    OperatorAuth:       eywa.NewOperatorAuth(operatorRepo, []byte(jwtSecret)),
    EchoRepo:           echoRepo,
    EchoQueryRepo:      echoRepo,
    ChronicleQueryRepo: chronicleRepo,
    WeaveConfigRepo:    eywamongo.NewWeaveConfigRepository(db),
    ConfigCache:        eywa.NewConfigCache(linkRepo, nil, nil),
    HTTPToolRepo:       eywamongo.NewHTTPToolRepository(db),
    VigilRepo:          vigilRepo,
    VigilConfig:        eywa.VigilConfig{InactivityTimeout: 30 * time.Minute},
    RiteRepo:           eywamongo.NewRiteRepository(db),
    ImprintRepo:        eywamongo.NewImprintRepository(db),
    PubSub:             eywaredis.NewPubSub(redisClient), // habilita SSE + fanout em tempo real
})
app.Listen(":8080")
```

Rotas registradas (todas sob `/api/v1`, auth obrigatória exceto `/auth/token`):

| Recurso | Rotas |
|---------|-------|
| 🔍 Discovery | `GET /discovery` — actions, scouts, classifiers, channels, routers registrados |
| 👻 Spirits | `GET/POST /spirits` · `GET/PUT/DELETE /spirits/:name` · `GET /spirits/:name/versions` |
| 📜 Chronicle | `GET /chronicle` · `GET /chronicle/:id` |
| 📊 Analytics | `GET /analytics/tokens` · `/analytics/actions` · `/analytics/spirits` |
| 💬 Conversas | `GET /echoes/sessions` · `GET /echoes/sessions/:key` · `POST /echoes/sessions/:key/messages` |
| 🧠 Imprints | `GET /imprints` (filtro por user/spirit/category) · `DELETE /imprints/:id` |
| ⚙️ Config | `GET/PUT /event-configurations/:eventType` · `GET/PUT /admin/engine-config` |
| 🔧 HTTP Tools | `GET/POST /http-tools` · `GET/PUT/DELETE /http-tools/:id` · `POST /http-tools/:id/test` |
| 🙋 Vigil | `GET /vigil` (todos ativos) · `POST/DELETE/GET /vigil/:memoryKey` · `POST /vigil/:memoryKey/echoes` |
| ✅ Rites | `GET /rites` · `GET /rites/:id` · `POST /rites/:id/approve` · `POST /rites/:id/reject` |
| 👤 Operators | `GET/POST /operators` · `GET/PUT/DELETE /operators/:id` |
| 📡 SSE | `GET /sse/rites` · `GET /sse/vigil` · `GET /sse/echoes/:memoryKey` |
| 🔑 Auth | `POST /auth/token` (público) |

---

## 📡 SSE em Tempo Real

Quando `PubSub` está configurado no `ManagementDeps`, a API de gerenciamento ganha três endpoints de Server-Sent Events. O cockpit e qualquer dashboard customizado podem assinar eventos de lifecycle sem polling.

```typescript
// Browser — assina eventos de lifecycle de Rite
const es = new EventSource('/api/v1/sse/rites', { withCredentials: true })
es.onmessage = (e) => {
    const { event, rite } = JSON.parse(e.data)
    if (event === 'rite_created') showApprovalToast(rite)
}
```

| Endpoint | Eventos |
|----------|---------|
| `GET /api/v1/sse/rites` | `rite_created` · `rite_decided` · `rite_expired` |
| `GET /api/v1/sse/vigil` | `vigil_acquired` · `vigil_released` |
| `GET /api/v1/sse/echoes/:memoryKey` | `message_added` · `vigil_acquired` · `vigil_released` · `rite_created` |

Suportado por Redis PubSub — todos os eventos chegam a todas as instâncias em execução. Conexão mantida viva com heartbeat de ping a cada 30 segundos.

---

## 🌿 eywa-cockpit — *O Grove está crescendo*

> *Nas profundezas do Weave, algo se move. As raízes de uma nova árvore estão se firmando — uma que permite enxergar cada Spirit, cada Pulse, cada Rite e assento Vigil através de uma única interface luminosa.*

**eywa-cockpit** é uma UI completa de gerenciamento para o engine Eywa, atualmente sendo cultivada. Construída sobre a mesma mitologia, traz cada conceito do sistema à vida na tela — em tempo real, interativa, viva.

O que está tomando forma:

- **Hometree** — dashboard vivo com gráficos de uso de tokens, saúde dos Spirits e Rites pendentes
- **Spirit Grove** — crie e versione Spirits com editor de configuração completo
- **Echo Chamber** — inspetor de conversas ao vivo com takeover Vigil e mensagens de operador
- **Vigil Watch** — todos os assentos de operador ativos, em tempo real via SSE
- **Rite Chamber** — fila de aprovação com aprovar/rejeitar em um clique
- **Chronicle** — log de auditoria completo com detalhe de interação e breakdown de custo
- **Pulse Flows** — configuração visual de roteamento de eventos
- **Conduit Gateway** — construtor de HTTP tools com test runner ao vivo

O Grove está sendo forjado. Quando abrir, cada equipe rodando Eywa terá uma janela para o seu Weave.

---

## 🧪 Exemplos

Todos os 13 exemplos são executáveis com apenas MongoDB, Redis e uma API key de LLM:

| Exemplo | Conceitos |
|---------|----------|
| [`01_basic_setup`](_examples/01_basic_setup/) | Weave mínimo — Spirit, Pulse, `ProcessEventByKey` |
| [`02_custom_actions`](_examples/02_custom_actions/) | Implementando e registrando Actions customizadas (tool use) |
| [`03_advanced_routing`](_examples/03_advanced_routing/) | Scouts + Pathfinders + roteamento multi-Spirit |
| [`04_async_concept`](_examples/04_async_concept/) | Comparação entre processamento síncrono e assíncrono |
| [`05_multi_provider`](_examples/05_multi_provider/) | Múltiplos provedores Oracle — Spirits em modelos diferentes |
| [`06_rag_with_lore`](_examples/06_rag_with_lore/) | RAG: ingira documentos, action `search_lore`, port `LoreEmbedder` |
| [`07_human_takeover`](_examples/07_human_takeover/) | Vigil: operador adquire/libera assento, `ErrSessionHeld` |
| [`08_approval_workflow`](_examples/08_approval_workflow/) | Rites: Spirit solicita aprovação, operador decide |
| [`09_long_term_memory`](_examples/09_long_term_memory/) | Imprint: `remember_fact`, extração automática, fatos cross-session |
| [`10_cost_tracking`](_examples/10_cost_tracking/) | Ledger: `TokenBudget`, `ModelRoutingRule`, estatísticas de uso |
| [`11_mcp_client`](_examples/11_mcp_client/) | Conduit: conecta a servidor MCP, auto-descobre ferramentas |
| [`12_management_api`](_examples/12_management_api/) | API Fiber completa com autenticação de operador |
| [`13_multi_agent`](_examples/13_multi_agent/) | Spirit Orquestrador: `summon_spirit`, `OrchestratorConfig` |

---

## 🏗️ Arquitetura

Eywa é construído sobre **arquitetura hexagonal** (ports & adapters). O domínio não tem dependências de infraestrutura — você troca MongoDB por Postgres, Redis por Valkey, OpenAI por Bedrock, sem tocar no engine.

```
┌────────────────────────────────────────────────────────────────┐
│                      Sua Aplicação                             │
├────────────────────────────────────────────────────────────────┤
│  Rotas Fiber │ Receptor WhatsApp │ Callback Cloud Tasks        │
├────────────────────────────────────────────────────────────────┤
│                    Weave (engine core)                          │
│   Pipeline · Memory · Archivist · Pathfinder · Actions          │
├──────────┬──────────┬──────────┬──────────┬────────────────────┤
│  MongoDB │  Redis   │  OpenAI  │ Bedrock  │  Qualquer MCP      │
│  adapter │  adapter │  Oracle  │  Oracle  │  (via Conduit)     │
└──────────┴──────────┴──────────┴──────────┴────────────────────┘
```

---

## 📄 Documentação

| Documento | Descrição |
|-----------|-----------|
| [🌐 Referência da API REST](docs/rest-api.md) | Todos os endpoints com body, parâmetros, respostas e eventos SSE |
| [🏗️ Arquitetura](docs/architecture.md) | Pipeline completo, entidades, fluxo de dados, estrutura hexagonal |
| [🔧 Referência do Builder](docs/builder.md) | Todas as opções do WeaveBuilder com exemplos |
| [📖 Conceitos e Interfaces](docs/concepts.md) | Como implementar cada ponto de extensão |
| [🧩 Sub-módulos](docs/sub-modules.md) | MongoDB, Redis, GCP, Fiber, WhatsApp, provedores LLM e vector stores |

> [!NOTE]
> A documentação técnica está em inglês — padrão da indústria para bibliotecas Go.

---

## 🤝 Contribuindo

Cada adapter que você escreve estende o Weave. Como tudo no Eywa é conectado através de interfaces, contribuir significa implementar um port e publicar como um sub-módulo independente — sem precisar entender os internals do engine.

**O que a comunidade pode construir:**

| Tipo | Interface | Exemplos |
|------|-----------|---------|
| 🔮 Oracle LLM | `eywa.Oracle` | Cohere, DeepSeek, Fireworks AI, Azure OpenAI, llama.cpp |
| 🔍 Vector Store | `eywa.LoreStore` | Chroma, Milvus, OpenSearch, Redis Vector Sets |
| 📣 Canal | `eywa.Voice` + `eywa.Receptor` | Telegram, Slack, Discord, SMS, Email, WeChat |
| 🗄️ Repositório | `eywa.*Repository` | PostgreSQL, DynamoDB, Firestore, Valkey |
| ☁️ Cloud | `eywa.Vault` + `eywa.Lens` | S3, Azure Blob, AWS Transcribe |

Veja [CONTRIBUTING.md](CONTRIBUTING.md) para estrutura, requisitos e como publicar.

### Rodando os testes

```bash
# Todos os testes
make test

# Testes + resumo de cobertura
make coverage

# Relatório HTML interativo de cobertura
make coverage-html
```

**Todo PR deve incluir testes.** Veja [CONTRIBUTING.md → Testing](CONTRIBUTING.md#testing) para as convenções usadas neste projeto.

---

## ☕ Apoie o projeto

Se o Eywa te poupou tempo — ou só fez você pensar diferente sobre infraestrutura de IA — considere comprar um café. Ajuda a manter o Weave crescendo.

<p align="center">
  <a href="https://github.com/sponsors/wmulabs">
    <img src="https://img.shields.io/badge/Sponsor_no_GitHub-%23EA4AAA?style=for-the-badge&logo=github" alt="GitHub Sponsors"/>
  </a>
  &nbsp;
  <a href="https://buymeacoffee.com/wmulabs">
    <img src="https://img.shields.io/badge/Buy_Me_a_Coffee-%23FFDD00?style=for-the-badge&logo=buymeacoffee&logoColor=black" alt="Buy Me a Coffee"/>
  </a>
</p>

---

## 📜 Licença

Apache 2.0 — veja [LICENSE](LICENSE).

---

<p align="center">
  <sub>🌿 Inspirado na rede neural de Pandora. Construído para sistemas de IA em produção em Go.</sub>
</p>
