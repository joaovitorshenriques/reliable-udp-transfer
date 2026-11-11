# Plano de implementação e avaliação

Escopo: obrigatório completo, depois dashboard e nuvem. A escolha inicial é Go; evitar frameworks e dependências até existir necessidade concreta. A entrega é em 15/12/2026 conforme o enunciado. O bônus total informado é até 10 pontos, sem distribuição por item.

| Etapa | Entrega | Como avaliar | Situação |
|---|---|---|---|
| 1 | Transporte confiável e CLI | Testes com dados/ACKs perdidos, corrupção, duplicação e reordenação; arquivo de 80 MiB com hash igual | Implementada; consultar VALIDACAO.md |
| 2 | Cliente-servidor | Senha correta aceita; senha incorreta rejeitada; listar arquivos; upload e download remotos; impedir caminhos fora da pasta; erros legíveis | Implementada em modo sequencial; avaliação manual em ETAPA2.md |
| 3 | Robustez e observabilidade | Sessões isoladas, limite de recursos, RTO adaptativo, métricas em JSON/CSV; falhas não travam serviço | Pendente |
| 4 | Experimentos | Repetições dos quatro cenários, arquivos >=80 MB, variação de janela, gráficos e capturas | Pendente |
| 5 | Mini-CDN | Duas máquinas, sondagens repetidas, escolha pelo menor RTT, captura da seleção, comparação de desempenho | Pendente |
| 6 | Dashboard web | RTT, throughput, perdas, retransmissões e janela vindos do processo real | Pendente |
| 7 | Nuvem IaaS | Servidor remoto autenticado, regra UDP explícita, comparação local/nuvem usando mesma configuração | Pendente |

## Recomendações de arquitetura

- Separar codec (`internal/protocol`), transporte (`internal/transfer`), aplicação (`internal/service`) e comandos (`cmd/rc2`). Próxima etapa: robustez e gerenciador de sessões.
- Manter o núcleo de transporte compartilhado pelos dois sentidos. Não duplicar confiabilidade no upload e no download.
- Usar estado explícito por sessão e uma rotina proprietária desse estado. Não criar uma goroutine por pacote.
- Reservar evolução de cabeçalho por versão; o atual possui ID de sessão, mas ainda não possui multiplexação de fluxos.
- Os comandos `list`, `upload` e `download` solicitam operações ao servidor; `send` e `receive` permitem validar o transporte isoladamente.
- Preparar a autenticação e os limites antes da nuvem. Não trocar a implementação própria por biblioteca QUIC pronta: isso esconderia os mecanismos avaliados.

## Experimentos obrigatórios

| Cenário | RTT alvo | Perda configurada |
|---|---:|---:|
| 1 | 25 ms | 0% |
| 2 | 100 ms | 0% |
| 3 | 100 ms | 5% |
| 4 | 400 ms | 5% |

Recomendo no mínimo cinco repetições por cenário/janela, mantendo arquivo, versão, máquina e parâmetros registrados. Testar janelas 16, 64 e 256 inicialmente, além de 1 como referência Stop-and-Wait. Registrar média e dispersão, e preservar amostras brutas. Não usar números simulados no relatório como se fossem medições.

RTT não é atraso por sentido: para simular RTT 100 ms em caminho simétrico, aproximadamente 50 ms por sentido, descontado o atraso de base. Adicionar `sleep` sequencial por pacote no emissor também limita a taxa de envio e pode invalidar a comparação; preferir emulador de enlace com filas/atraso independente. Definir se os 5% são por sentido e se incluem ACKs. Medir o resultado efetivo.

Wireshark ainda não foi instalado/verificado. Solicitar autorização se necessário. Capturar `udp.port == 9000` em teste local; procurar magic `RC2U`, tipos DATA/ACK e mesma sessão/sequência em retransmissões. A captura do loopback no Windows pode exigir driver/ferramenta adicional; não instalar automaticamente.

## Nuvem e extras

Ordem recomendada: dashboard, nuvem, estabelecimento rápido, multiplexação, criptografia, migração. A implantação dependerá da conta, orçamento e provedor escolhidos pelo usuário. Nenhum recurso pago será criado nesta etapa.

Para a comparação, usar a mesma versão, arquivo e janela nos dois ambientes; registrar região, configuração da VM, RTT real e regras de rede. Diferenciar resultados de emulação controlada dos efeitos reais da Internet.

## O que cada integrante deve saber explicar

1. Por que UDP exige ACK, número de sequência e retransmissão.
2. Como a janela limita pacotes em trânsito e memória do receptor.
3. O que acontece com um ACK perdido ou dado duplicado.
4. Diferença entre CRC32, SHA-256 e autenticação.
5. Por que throughput/goodput e RTT afetam a escolha de janela.
6. Como os experimentos foram realizados e quais limitações permanecem.

Ao terminar cada etapa, rodar os critérios, guardar evidências e explicar os resultados entre os integrantes antes de expandir o escopo.
