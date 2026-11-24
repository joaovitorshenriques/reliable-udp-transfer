# Encerramento da etapa 4 — 14/11/2026

Evidência: saída completa fornecida pelo usuário de `go run ./cmd/validate -large -stage4 -cross`. Todos os testes habilitados passaram, assim como `go vet ./...`, build dos comandos Windows e compilação Linux. Não houve execução de testes no Linux.

- TestStage4UDP: quatro cenários aprovados, upload/download de 1 MiB com hash correto; total 97,19 s.
- TestLargeFile: 83.886.080 bytes, SHA-256 `42d12a7bb4fe5161f36dd20908f31ea01e2ae86741f32adc970dc527f22ad4f4`, 1,955 s de transferência, goodput 343,200 Mbit/s, zero retransmissões.
- Concorrência, interrupção/recuperação, autenticação, integridade, buffers e RTO aprovados. Logs de cancelamento/timeout pertencem aos testes que provocam essas condições; o log de socket fechado em TestUploadDownload ocorreu com teste aprovado e permanece uma limitação de limpeza/log já registrada, não uma falha da suíte.
- Campanhas: janelas 16/64/256, cinco repetições, quatro cenários, 120 transferências de 80 MiB verificadas. Comparação e IC95% disponíveis em comparacao-etapa4/comparison.md.
- Captura S3: 3.773 pacotes com CRC válido, 143 DATA repetidos observáveis e zero descartes informados pelo capturador.

Etapa 4 concluída no escopo de emulação local documentado. Mini-CDN entre máquinas pertence à etapa 5. Esta atualização é documental; nenhum commit ou push foi realizado.

## Histórico de verificações anteriores

## Consolidação experimental — 14/11/2026

Campanhas de 80 MiB com janelas 16, 64 e 256 concluídas e verificadas: 120 transferências, 240 registros de métricas e 60 pares do emulador. Hashes/tamanhos e CSV/JSONL conferidos; zero descartes por fila e erros de envio. Comparação, IC95% e gráficos gerados em [comparacao-etapa4/comparison.md](comparacao-etapa4/comparison.md). Captura S3 e decodificação documentadas em CAPTURA-S3.md e capture-s3/capture.md.

A janela 256 apresentou o maior goodput médio nos quatro cenários e nos dois sentidos. Em upload, o ganho 256/16 foi aproximadamente 15,72 vezes em S1, 15,81 em S2, 7,00 em S3 e 7,14 em S4. Com perdas, o ganho foi menor que a razão entre as janelas (16). Isso é compatível com custos de recuperação e bloqueio da base da janela; não isola causalmente cada custo. Há retransmissões mesmo sem perda injetada: não atribuí-las automaticamente a perda física. Os resultados são de loopback com emulação, não de enlace real entre máquinas.

As campanhas ocorreram sequencialmente e os manifestos não registram hardware/commit; igualdade desses fatores depende do controle da execução pelo operador. Não foi realizado teste de significância entre janelas. A validação completa combinada `go run ./cmd/validate -large -stage4 -cross` foi aprovada no Windows em 14/11/2026, com build Linux aprovado (sem execução Linux). Não repetir campanhas longas já aprovadas.

## Ferramentas de fechamento da etapa 4

Decodificador Lua carregado com sucesso pelo TShark 4.6.9 na captura existente; campos rc2.seq/rc2.kind reconhecidos. Inspeção Python validou CRC, versão e tamanho dos 3.773 pacotes e identificou 143 DATA repetidos. Scripts Python passaram na compilação sintática. Comparador rejeitou corretamente três entradas da mesma janela (64), sem gerar comparação inválida. O caminho completo de comparação aguarda as campanhas reais das janelas 16 e 256. Não houve repetição da campanha longa nem nova captura nesta verificação.

# Registro de validação — 10/11/2026

## Etapa 4 — campanha de 80 MiB/janela 64 (13/11/2026)

Analisados os arquivos de `results/etapa4-1490099931`: COMPLETE presente, 80 registros de transporte, 40 sessões pareadas e 20 registros de emulação. Todos os hashes/tamanhos/sucessos corretos; configurações e sementes conferidas com o manifesto; CSV/JSONL consistentes. Zero descartes por fila cheia, erros de envio ou pendentes/cancelados. Cinco repetições por cenário, upload e download.

Resumo e IC95% t de Student em [comparacao-etapa4/w64/analysis.md](comparacao-etapa4/w64/analysis.md), tabela numérica em `comparacao-etapa4/w64/summary.csv`. Médias de goodput do emissor (upload/download): S1 23,3184/23,2777; S2 5,9464/5,9463; S3 2,2430/2,2097; S4 0,5721/0,5684 Mbit/s. RTT médio próximo de 25,7/100,7/100,8/400,8 ms. Em S3/S4, perdas observadas por sentido entre 4,9849% e 5,0370%.

S2 apresentou 918 retransmissões no upload e 3162 no download (totais das cinco repetições), apesar de nenhuma perda sorteada. A causa não foi comprovada: requer captura/análise de temporização; não inferir perda física a partir desse contador. S1 não apresentou retransmissões. O resumo usa o emissor em ambas as operações; os números de download do terminal eram do receptor.

Os resumos e gráficos da campanha estão em [comparacao-etapa4](comparacao-etapa4/comparison.md).

## Etapa 4 — integração UDP aprovada (13/11/2026)

Evidência: saída completa fornecida pelo usuário para `go run ./cmd/validate -stage4 -cross`. Todos os testes habilitados passaram, assim como vet e builds Windows/Linux. `TestStage4UDP` completou upload/download de 1 MiB nos quatro cenários em 104,54 s, com hashes corretos, relatórios pareados e verificações do emulador aprovadas. TestLargeFile foi ignorado intencionalmente porque `-large` não foi solicitado.

| Cenário | Upload: duração cliente emissor (s) | Download: duração cliente receptor (s) |
|---|---:|---:|
| S1 | 0,421 | 0,402 |
| S2 | 1,626 | 1,526 |
| S3 | 4,994 | 5,378 |
| S4 | 15,206 | 17,796 |

Estas são amostras únicas de testes de integração, com limites de medição distintos entre emissor/receptor. Não são resultados finais nem justificam conclusões estatísticas. Próximo passo: campanha de cinco repetições por cenário, 80 MiB e janela 64, preservando dados brutos. Variação de janela, análise, gráficos e capturas permanecem pendentes. Não foi feito commit da etapa 4.

## Etapa 4 — implementação inicial (13/11/2026)

Criado emulador de datagramas com atraso agendado, perda Bernoulli por sentido e semente, filas limitadas e contadores distintos de descarte sorteado/fila/erro de envio. Passaram testes sem sockets de atraso mínimo, encaminhamento de rajada sem serializar o atraso, limites de perda 0/100%, repetibilidade do sorteio, contabilidade de fila e fechamento. Os cenários S1–S4 e geração de arquivos também passaram nos testes unitários. `go vet ./...` e builds Windows/Linux passaram.

O teste UDP de integração `TestStage4UDP` é opt-in e não foi executado nesta validação automatizada. Próximo comando no terminal local: `go run ./cmd/validate -stage4 -cross`. Arquivos de 1 MiB são apenas testes de integração. Ainda não há campanha de 80 MiB, gráficos ou capturas da etapa 4. Nenhum resultado anterior aprova automaticamente o novo emulador.

## Etapa 3 concluída — 12/11/2026

Evidência: saída completa do usuário para `go run ./cmd/validate -large -cross`, após a implementação concorrente. Todos os testes passaram, incluindo clientes simultâneos em memória e UDP real, limite de endpoints, expiração, relatórios pareados, recuperação, autenticação e transporte. Análise estática e compilações Windows/Linux também passaram. Linux foi verificado por compilação, conforme o escopo acordado, sem execução nativa.

O teste grande transferiu 83.886.080 bytes com SHA-256 `42d12a7bb4fe5161f36dd20908f31ea01e2ae86741f32adc970dc527f22ad4f4`, duração no emissor de 1,786 s, goodput de 375,661 Mbit/s e zero retransmissões. Duração total: 8,94 s. Esse teste grande usa o transporte diretamente; a concorrência do serviço foi validada separadamente com arquivos pequenos. A campanha A/B/C continua sendo evidência histórica anterior ao dispatcher, sem nova alegação de desempenho concorrente.

Mensagens de falha em `stalled` e `partial.bin` são esperadas dos testes de interrupção. A mensagem `use of closed network connection` em `TestUploadDownload/upload` ocorre durante a limpeza: o teste já confirmou o conteúdo, mas o encerramento do dispatcher fecha a conexão virtual enquanto o receptor ainda aguarda FINISH repetido. O teste passa; essa revisão pode registrar erro durante o encerramento mesmo depois de publicar o arquivo. Não interpretar essa mensagem isoladamente como corrupção ou perda do arquivo.

Etapa 3 encerrada no escopo documentado: atendimento limitado a oito endpoints, RTO adaptativo, observabilidade, recuperação e comandos multiplataforma. Próxima entrega: etapa 4, quatro cenários controlados de RTT/perda, gráficos e capturas. Race detector e campanha de desempenho concorrente não foram executados.

## Concorrência — revisão em validação

Implementado dispatcher por endpoint com limites de clientes/fila, descarte quando fila cheia, expiração de endpoints ociosos e relatórios serializados. O teste em memória de clientes simultâneos passou no ambiente automatizado usando temporários em `work/test-tmp`; a primeira execução no diretório temporário padrão foi impedida de publicar arquivos por `Access is denied`. Isso não foi tratado como aprovação. A suíte UDP completa da revisão concorrente ainda deve ser executada localmente. Não reutilizar a campanha A/B/C anterior como evidência de desempenho do novo dispatcher.

## Robustez aprovada no Windows — 12/11/2026

Evidência: saída completa do usuário ao executar `go run ./cmd/validate -cross`. Todos os testes habilitados passaram, incluindo `TestInterruptedUploadRecovery/idle` e `/cancel`, cancelamento com RTO longo e preservação do prazo de retransmissão. As mensagens `receiver idle timeout` e `context canceled` são falhas provocadas deliberadamente nos testes e não falhas da suíte. Foram verificadas limpeza de arquivos parciais, resposta de ocupado e recuperação após inatividade.

Também passaram `go vet`, build Windows e build cruzado Linux. `TestLargeFile` foi ignorado intencionalmente porque `-large` não foi solicitado; o teste grande e a campanha A/B/C anteriores continuam registrados abaixo, sem alegar repetição nesta revisão. O servidor permanece sequencial, sem suporte a transferências simultâneas. Nenhum commit foi feito.

## Robustez posterior à campanha — validação pendente de UDP

Após a campanha A/B/C foram ajustados cancelamento nas esperas de controle, resposta de ocupado a novos pedidos da mesma porta e timeout por dados novos no receptor. Os resultados anteriores referem-se à versão anterior a esses ajustes. Foram adicionados testes de cancelamento/RTO sem sockets e integração de upload interrompido, limpeza e recuperação. A integração precisa ser executada no terminal local: `go run ./cmd/validate -cross`. Não se deve registrar a nova revisão como aprovada apenas com a campanha anterior.

Passaram nesta revisão: `TestControlCancellationWithLongRTO` (0,10 s), `TestControlPollingPreservesRTO` (0,50 s), `go vet ./...`, build Windows, build Linux e `git diff --check`. O pacote de testes de serviço compilou (`go test ./internal/service -run '^$'`); esse comando não executa os testes de integração.

## Campanha local da etapa 3 — 11/11/2026

Campanha `results/etapa3-1499839470`, executada pelo usuário no Windows, Go 1.27.4: concluída, marcador COMPLETE presente. Conferidos 60 registros, 30 sessões com dois registros por sessão, todos com sucesso e 83.886.080 bytes. Hash único igual ao manifesto: `42d12a7bb4fe5161f36dd20908f31ea01e2ae86741f32adc970dc527f22ad4f4`.

Médias aritméticas de goodput do **emissor**, cinco repetições por operação/grupo:

| Grupo | Upload (Mbit/s) | Download (Mbit/s) | Retransmissões DATA totais upload/download |
|---|---:|---:|---:|
| A — fixo, buffers SO | 342,64 | 318,59 | 4 / 8 |
| B — fixo, buffers 4 MiB | 348,41 | 324,69 | 0 / 0 |
| C — adaptativo, buffers 4 MiB | 335,62 | 342,52 | 0 / 0 |

Integridade e métricas pareadas aprovadas nesta campanha local. B e C não apresentaram retransmissões; C não foi uniformemente mais rápido que B. São resultados descritivos de cinco repetições em loopback, sem perdas injetadas, com cliente/servidor no mesmo processo; não demonstram superioridade geral nem substituem os quatro cenários do enunciado. Retransmissões não medem diretamente perda de rede.

A parte de temporização/observabilidade da etapa 3 foi validada. O plano mais amplo ainda inclui robustez, isolamento de sessões e limites de recursos; o servidor continua sequencial. Próximo passo: revisar comportamento de cancelamento, falha e atendimento de pedidos durante uma transferência antes de ampliar os experimentos.

## Executor da comparação da etapa 3

Adicionado `cmd/experiment`, somente biblioteca padrão Go, para comparar A/B/C com upload/download, integridade relida do disco, métricas pareadas JSONL/CSV e manifesto. Passaram os testes do gerador/verificador de integridade (incluindo corrupção) e da rejeição de configurações inválidas, `go vet ./...`, compilação Windows, compilação cruzada Linux e `git diff --check`. A campanha UDP deste novo executor ainda não foi executada: deve rodar no PowerShell local, onde a comunicação já foi validada. Não há resultados comparativos a relatar ainda.

Escopo atualizado pelo usuário: manter compatibilidade Windows/Linux, sem exigir execução nativa Linux. Compilação cruzada permanece como verificação de build, sem alegação de teste de execução Linux. As pendências históricas abaixo devem ser lidas à luz desta decisão.

## Etapa 3 — validação funcional aprovada no Windows

Evidência: saída completa fornecida pelo usuário ao executar `go run ./cmd/validate -large -cross` no PowerShell local, com Go 1.27.4 windows/amd64. A suíte inteira passou: buffers UDP, codec e sementes de fuzz, autenticação/listagem, upload/download, relatórios pareados, RTO adaptativo e fixo, perdas/reordenação/duplicação/corrupção, publicação sem sobrescrita e hash incorreto. `go vet`, build Windows e build cruzado Linux também passaram.

Teste de transporte de 80 MiB, com buffers solicitados de 4 MiB e RTO adaptativo:

- 83.886.080 bytes; SHA-256 `42d12a7bb4fe5161f36dd20908f31ea01e2ae86741f32adc970dc527f22ad4f4`, integridade conferida.
- Duração medida no emissor: 1,650 s; goodput: 406,615 Mbit/s; retransmissões de dados: 0.
- Duração total do teste: 8,80 s, incluindo preparação e espera final do receptor.

O teste UDP independente em PowerShell local também passou. As falhas anteriores no ambiente automatizado não se reproduziram no terminal do usuário. A causa específica da restrição automatizada não foi identificada.

Validação funcional Windows aprovada; ainda pendentes execução nativa Linux, upload/download de 80 MiB com métricas pareadas e comparação dos grupos A/B/C (cinco repetições por grupo). Esta única medição local não comprova ganho causado pelo RTO/buffers e não é comparável diretamente às medições da máquina anterior. Nenhum commit foi realizado.

## Retomada com Go instalado — 11/11/2026

Go 1.27.4 windows/amd64 encontrado em `C:\Program Files\Go\bin\go.exe` após instalação pelo usuário. Executado `go run ./cmd/validate -large -cross`: os processos de teste iniciaram, mas a suíte falhou por timeouts UDP. Passaram codec/sementes de fuzz, estimador RTO, validação da configuração, nomes, recusa de buffers inválidos, preservação/publicação sem sobrescrita e receptor indisponível. Falharam comunicação UDP, serviço, transferências e o teste de 80 MiB, que não chegou a transferir o arquivo.

Uma prova independente com dois `System.Net.Sockets.UdpClient` em PowerShell também falhou ao receber uma mensagem em `127.0.0.1`, com timeout de dois segundos. Isso indica uma limitação de comunicação no ambiente de execução; a causa específica (sandbox, política ou outra configuração) não foi determinada. Nenhuma proteção foi alterada. Não há evidência para aprovar a etapa 3 ou medir desempenho.

`go vet ./...`, compilação Windows e compilação cruzada Linux foram executados separadamente com sucesso. A execução nativa Linux continua pendente. Para verificar fora do ambiente automatizado, executar no terminal local, na raiz do projeto: `go run ./cmd/validate -large -cross`. A execução interrompe no primeiro erro e não converte falhas de rede em testes ignorados.

## Retomada em outro Windows — 11/11/2026

A etapa 3 permanece **pendente de execução** nesta máquina. Na retomada, `.tools/go` não estava presente e não havia `go` no PATH. A tentativa autorizada de obter o compilador portátil em `go.dev` falhou na resolução do host; nenhum compilador foi instalado. Não se deve confundir esse impedimento com o bloqueio por Controle de Aplicativo registrado na máquina anterior.

Adicionados um executor em Go (`go run ./cmd/validate`, opções `-large` e `-cross`), wrapper opcional POSIX e testes de comunicação UDP com buffers padrão/4 MiB. Os testes usam sockets loopback, portas efêmeras e diretórios temporários, sem comandos específicos de Windows. `.gitattributes` define LF para Go/Markdown/shell e CRLF para PowerShell; não foi feita renormalização do índice nem commit.

Nenhum dos testes novos, compilação cruzada ou benchmark foi executado nesta retomada por falta do compilador. Os resultados históricos abaixo pertencem às execuções anteriores.

`git diff --check` passou nesta retomada. A verificação de sintaxe do wrapper com o Bash do Git também ficou bloqueada: o processo falhou ao criar um objeto do Windows (`NtCreateDirectoryObject`, `0xC0000022`), antes de analisar o script. Isso não constitui aprovação nem falha de sintaxe do wrapper.

Ambiente: Windows amd64, Go 1.27.4 portátil, rede loopback, execução pelo ambiente do Codex. Dependências externas de Go: nenhuma. Estes resultados são verificações de engenharia da etapa 1, não os quatro cenários exigidos no relatório.

## Executado

| Verificação | Resultado |
|---|---|
| `go test ./... -count=1 -v` | Suíte funcional passou; teste grande é opt-in |
| `go vet ./...` | Passou, sem diagnósticos |
| `go build -o bin/rc2.exe ./cmd/rc2` | Executável gerado |
| CLI em dois processos UDP, arquivo de 15 bytes | Sucesso no emissor/receptor, hashes iguais |
| `RC2_LARGE_TEST=1 go test ./internal/transfer -run TestLargeFile -v -count=1 -timeout 3m` | Passou com 83.886.080 bytes |

## Transferência de 80 MiB

- Bytes recebidos e conferidos: **83.886.080**.
- SHA-256: `42d12a7bb4fe5161f36dd20908f31ea01e2ae86741f32adc970dc527f22ad4f4`.
- Janela configurada: 64 pacotes.
- RTO fixo: 500 ms.
- Tempo medido no emissor: 46,304 s.
- Goodput do emissor: 14,493 Mbit/s.
- Retransmissões de dados: 636.
- Duração total do teste: 53,55 s, incluindo criação do arquivo e espera final do receptor.

Não foi injetada perda nesse teste grande. Ainda assim houve retransmissões; **não atribuir automaticamente isso à perda no enlace**. Descarte em buffers locais e atrasos de processamento/agendamento são hipóteses a investigar com captura e instrumentação. É uma amostra única neste ambiente, sem significado de benchmark comparativo. A próxima etapa de desempenho deverá investigar buffers de socket, RTO e custo por pacote.

## Correção encontrada durante os testes

A tentativa inicial de publicar arquivos por hard link falhou neste ambiente Windows. A implementação final usa `MoveFile` no Windows, que não substitui destino existente; nos outros sistemas mantém hard link. Os testes de transferência foram executados novamente após a correção. Há teste específico para garantir que a publicação recusa um arquivo criado antes dela.

## Ainda não validado

- Linux/macOS e transferência entre máquinas reais.
- Cenários com RTT 25/100/400 ms e perda controlada de 5%.
- Variação de janela e análise estatística.
- Capturas Wireshark, race detector e campanha prolongada de fuzzing (somente sementes do fuzz foram executadas).
- Na etapa 1, autenticação e serviço de upload/download ainda estavam pendentes. A validação da etapa 2 está registrada abaixo. Mini-CDN, dashboard e nuvem permanecem pendentes.

## Reproduzir

Siga os comandos do README. Os testes comuns criam e removem seus próprios arquivos temporários. O teste de 80 MiB demanda espaço temporário para duas cópias do arquivo. Os arquivos de smoke test da CLI estão em `work/` e não devem ser entregues como código-fonte.

## Etapa 2 — 11/11/2026

Implementados os comandos `serve`, `list`, `upload` e `download`, com senha via `RC2_PASSWORD`, operação sequencial na mesma porta UDP e restrição de nomes à pasta do servidor.

Verificações executadas:

- `go test ./... -count=1`: passou, incluindo os testes anteriores de transporte.
- Listagem paginada de 13 arquivos, exclusão de temporários ocultos e recusa de senha incorreta: passou.
- Upload e download em sockets UDP de loopback, com a primeira resposta do serviço descartada: passaram, com conteúdo idêntico.
- Recusa de arquivo inexistente, destino existente, caminho relativo externo, caminho Windows absoluto, alternate data stream e operação desconhecida: passou.
- `go vet ./...`: passou.
- Compilação do executável: passou.
- `git diff --check`: passou.

A execução da CLI no ambiente automatizado foi bloqueada pela política de Controle de Aplicativo do Windows. Posteriormente, a avaliação manual foi realizada pelo usuário em dois terminais PowerShell, com os resultados abaixo. O teste automatizado opt-in de 80 MiB não foi repetido nesta etapa.

### Validação manual local

Evidência: saídas dos terminais fornecidas pelo usuário em 11/11/2026.

- Listagem inicial vazia e listagem após upload: corretas.
- Upload e download de arquivo de 35 bytes: sucesso, hashes iguais.
- Senha incorreta, arquivo inexistente, sobrescrita e caminho `../fora.txt`: rejeitados com as mensagens esperadas.
- Upload e download de 83.886.080 bytes (80 MiB): sucesso, tamanhos e hashes iguais.
- SHA-256 do arquivo grande: `3428d511eb4e708633538b5b7c3b22535bda0a14391959a562d3e3148fbcbb02`.

| Métrica | Upload (cliente emissor) | Download (cliente receptor) |
|---|---:|---:|
| Tempo medido | 80,1097554 s | 63,6383926 s |
| Goodput | 8,377115 Mbit/s | 10,545342 Mbit/s |
| Datagramas de dados enviados | 72.657 | Não medido no emissor |
| Retransmissões de dados | 1.081 | Não medido no emissor |
| RTT médio | 2,32298 ms | Sem amostras |
| Amostras de RTT | 70.495 | 0 |
| Janela | 64 pacotes | 64 pacotes |

A validação funcional local da etapa 2 foi concluída. Os zeros de retransmissões e RTT exibidos pelo cliente no download não descrevem o comportamento do servidor emissor. Os tempos de emissor e receptor têm limites de medição diferentes, descritos no README.

As retransmissões em loopback precisam ser investigadas na etapa 3: buffers, agendamento/processamento e temporização são hipóteses, não causas confirmadas. Estes resultados são uma referência local de uma execução, não uma medição da taxa de perda nem os cenários experimentais finais.

## Etapa 3 — em validação (11/11/2026)

- Implementados: RTO adaptativo e modo fixo, escrita em blocos, configuração de buffers UDP, métricas pareadas por sessão, JSONL e exportação CSV.
- `go build ./...`: passou.
- `go vet ./...`: passou.
- `git diff --check`: passou.
- Conversão JSONL para CSV verificada com registro sintético de 123 bytes: campos preservados. Esse registro não é resultado experimental.
- Sintaxe do script de diagnóstico PowerShell: válida. Leitura efetiva dos eventos negada ao ambiente; requer execução local autorizada.
- Testes do codec e sementes de fuzz passaram. Os novos executáveis `service.test.exe` e `transfer.test.exe` foram bloqueados pelo Controle de Aplicativo, inclusive na tentativa do usuário no PowerShell. A suíte completa não está aprovada.
- Não foi feita nova medição de 80 MiB nem comprovada redução de retransmissões.

O roteiro `ETAPA3.md` descreve o diagnóstico e os critérios para continuar. WSL foi consultado e não está instalado; nenhuma dependência foi instalada nem política de proteção alterada.

Diagnóstico posterior: o usuário forneceu eventos 3077 com Policy ID `{0283ac0f-fff1-49ae-ada1-8a933130cad6}`, bloqueando `service.test.exe`, `transfer.test.exe` e `rc2.exe`. A [documentação da Microsoft](https://learn.microsoft.com/pt-br/windows/security/application-security/application-control/app-control-for-business/operations/inbox-appcontrol-policies) identifica esse ID como `VerifiedAndReputableDesktop`, política do Smart App Control. A causa do bloqueio foi identificada; a execução da suíte continua pendente. Um ambiente Linux/WSL é uma alternativa de validação, condicionada à autorização para instalar e sem substituir a validação específica do Windows.

## Etapa 5: captura conferida

15 sessões pareadas; 50 sondagens respondidas; cinco escolhas pelo menor RTT mediano elegível. START/primeiro DATA/DONE conferidos. Erro de admissão no frame 1720028 preservado. Inspeção dirigida dos pacotes aprovada; timestamps usados porque frames das duas interfaces não estão necessariamente em ordem temporal. Etapa 5 encerrada no escopo LAN documentado.
