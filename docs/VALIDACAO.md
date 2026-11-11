# Registro de validação — 10/11/2026

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
