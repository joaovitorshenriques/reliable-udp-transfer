# RC2 — transferência confiável sobre UDP

Projeto acadêmico CSI510 (Redes de Computadores II), implementado em Go com biblioteca padrão. O protocolo é próprio: cabeçalho binário, CRC32, ACKs individuais, janela deslizante Selective Repeat, retransmissões, RTO adaptativo e integridade SHA-256. Não utiliza uma biblioteca QUIC pronta.

## Sistemas e requisitos

Go 1.22 ou superior. Os comandos `go run`, `go test` e `go build` abaixo são iguais no Windows e no Linux. `go run` compila para o sistema atual e executa o programa; não exige scripts PowerShell ou Bash. Um binário compilado, por outro lado, é específico do sistema e arquitetura. Quem recebe o binário pronto não precisa instalar Go.

Execute os comandos na raiz do repositório. A senha pode vir de `RC2_PASSWORD` ou de `-password-file`. Para usar exatamente os mesmos comandos nos dois sistemas, crie com seu editor um arquivo UTF-8 chamado `password.txt`, contendo apenas uma senha de teste em uma linha. Esse arquivo é ignorado pelo Git. Não compartilhe o arquivo nem inclua credenciais no relatório.

## Executar o servidor

No primeiro terminal:

```text
go run ./cmd/rc2 serve -addr 127.0.0.1:9000 -dir data/server -password-file password.txt -metrics results/server.jsonl
```

Para clientes em outra máquina, use `-addr 0.0.0.0:9000` no servidor e o IP real do servidor no cliente, em uma rede de teste autorizada. Encerre com Ctrl+C.

## Executar o cliente

Em outro terminal, usando o README como pequeno arquivo de teste:

```text
go run ./cmd/rc2 list -addr 127.0.0.1:9000 -password-file password.txt
go run ./cmd/rc2 upload -addr 127.0.0.1:9000 -file README.md -name exemplo.md -password-file password.txt -metrics results/client.jsonl
go run ./cmd/rc2 download -addr 127.0.0.1:9000 -name exemplo.md -file recebido.md -password-file password.txt -metrics results/client.jsonl
```

Use nomes novos ao repetir: arquivos existentes nunca são substituídos. As saídas JSON indicam sucesso, hash e métricas. Cada comando cliente usa seu próprio socket. O receptor permanece por cerca de sete segundos após publicar o arquivo para responder FINISH repetido; essa espera não entra no goodput do receptor.

## Atendimento simultâneo e limites

O servidor despacha datagramas de uma única porta UDP para até **oito endpoints autenticados (IP e porta)**, cada um com estado, prazo de leitura e fila próprios. Cada endpoint executa uma operação por vez; um pedido novo do mesmo endpoint durante uma transferência recebe ocupado. Outros endpoints podem transferir ou listar simultaneamente. Não há multiplexação de vários fluxos dentro de uma única sessão.

Cada fila comporta 256 datagramas; quando cheia, novos datagramas são descartados e o transporte depende de retransmissões. O limite padrão por arquivo é 1 GiB, ajustável com `-max-size`; janela padrão de 64 pacotes. A publicação sem sobrescrita também protege duas tentativas concorrentes de enviar o mesmo nome: no máximo uma publica o destino.

Endpoints ociosos são liberados após aproximadamente dois minutos com a configuração padrão; esse período preserva o cache de pedidos concluídos. Os oito slots incluem endpoints ociosos ainda retidos, portanto várias invocações rápidas da CLI podem atingir o limite. Quando cheio, o servidor informa limite de clientes. O prazo de retenção aumenta se os limites de temporização configurados forem maiores. Não há limite global de armazenamento nem quota de disco por usuário.

## Validar

```text
go run ./cmd/validate -cross
```

Executa testes, análise estática e compilação nativa, além de compilação para o outro sistema. A opção `-cross` verifica o build; não executa testes no outro SO. Para incluir a transferência de 80 MiB:

```text
go run ./cmd/validate -large -cross
```

O cenário de concorrência tem uma versão com datagramas em memória e outra com sockets UDP reais. A suíte também cobre corrupção, perdas, reordenação, duplicatas, autenticação, arquivo existente, hash incorreto, cancelamento, limpeza de temporários e recuperação. Falhas de UDP do ambiente não são convertidas em testes ignorados. A etapa 3 foi aprovada em 12/11/2026 com a suíte completa no Windows, incluindo concorrência UDP e 80 MiB, vet e compilação Linux. A campanha A/B/C é anterior ao dispatcher; não mede o desempenho desta revisão concorrente.

## Campanha local da etapa 3

```text
go run ./cmd/experiment
```

Executa cinco repetições de upload/download de 80 MiB para cada grupo: A (RTO fixo/buffers padrão), B (fixo/4 MiB) e C (adaptativo/4 MiB). Grava manifesto, métricas JSONL/CSV e marcador `COMPLETE` em uma pasta nova de `results/`. É loopback com cliente e servidor no mesmo processo, sem perda injetada. Não substitui os quatro cenários obrigatórios nem a mini-CDN em máquinas diferentes. Veja [ETAPA3.md](docs/ETAPA3.md).

## Compilar para distribuição

```text
go build -o bin/ ./cmd/rc2
```

O Go gera `rc2.exe` no Windows e `rc2` no Linux. Os comandos `go run` acima são a opção comum aos dois sistemas; executar diretamente o binário usa a convenção de caminhos do terminal local. Compilador portátil também é aceito; os wrappers opcionais em `scripts/` não são requisitos do projeto.

## Métricas e limitações

`goodput_mbps` mede bytes úteis do arquivo por duração. No emissor, inclui negociação e confirmação final, mas não o hash prévio; no receptor, termina após publicar o arquivo. Use `session`, `operation`, `endpoint` e `role` para correlacionar as pontas. RTT e retransmissões descrevem o emissor; zeros no receptor não são medições do emissor. Retransmissões não medem diretamente perda da rede.

`transport_bytes_*` conta bytes RC2 (cabeçalho e payload), sem UDP/IP/Ethernet e sem comandos de serviço. Datagramas descartados antes do transporte, como fila de cliente cheia, não são contabilizados como recebidos pelo transporte. Não há medição de throughput no fio nesta etapa.

A senha trafega sem criptografia. Use apenas loopback ou rede isolada; ainda não exponha o servidor à Internet. CRC32 e SHA-256 não autenticam remetentes. `send`/`receive` são comandos diretos de transporte sem autenticação. Mantenha o diretório do servidor sob controle de usuários locais confiáveis.

Não há controle de congestionamento ou retomada de arquivos. A publicação no Windows usa MoveFile sem substituição; no Linux usa hard link e depende de suporte do filesystem. O temporário fica no diretório de destino. Uma interrupção prolongada após publicação pode deixar o emissor sem confirmação mesmo com o arquivo já salvo.

## Documentação

- [Plano e próximos passos](docs/PLANO.md)
- [Formato do protocolo](docs/PROTOCOLO.md)
- [Evidências de validação](docs/VALIDACAO.md)
- [Histórico da etapa 2](docs/ETAPA2.md)
- [Etapa 3 e campanha](docs/ETAPA3.md)

Experimentos controlados, mini-CDN, dashboard e nuvem são as próximas entregas. Não interpretar resultados históricos como aprovação de alterações posteriores.
