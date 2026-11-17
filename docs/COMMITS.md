# Organização dos commits

A sessão do assistente não pode gravar `.git/index.lock`. Execute no seu terminal, com índice inicialmente vazio:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File scripts/commit-stage4.ps1
```

O script cria quatro commits: emulador; cenários/validação; ferramentas de análise/captura; documentação/resultados. Ele usa caminhos explícitos e não inclui a etapa 5. Para em caso de erro; não o repita integralmente após sucesso parcial sem conferir `git log` e `git status`.

Depois de validar a etapa 5, estes grupos podem ser commitados separadamente:

```text
git add -- internal/service/service.go internal/service/probe.go internal/service/probe_test.go
git commit -m ":sparkles: feat(service): adicionar sondagem autenticada sem retransmissao"
git add -- internal/cdn cmd/rc2/main.go cmd/rc2/cdn.go
git commit -m ":sparkles: feat(cdn): selecionar replica por RTT mediano e baixar arquivo"
git add -- docs/ETAPA5.md docs/COMMITS.md
git commit -m ":memo: docs: orientar verificacao da mini-CDN entre maquinas"
```

Nenhum push é realizado por esses comandos. Os dados brutos em results estão ignorados: preserve essa pasta separadamente, incluindo PCAPs e manifestos; os relatórios commitados incluem hashes de proveniência.
