# Protocolo RC2U, versão 1

Datagramas binários com inteiros em ordem de rede (big-endian). Máximo de 1200 bytes: 28 de cabeçalho + até 1172 de payload. Esse tamanho evita fragmentação em muitos caminhos comuns; não implementamos descoberta de MTU.

| Offset | Bytes | Campo |
|---:|---:|---|
| 0 | 4 | Magic ASCII RC2U |
| 4 | 1 | Versão: 1 |
| 5 | 1 | Tipo |
| 6 | 2 | Reservado: zero |
| 8 | 8 | ID de sessão aleatório |
| 16 | 4 | Sequência |
| 20 | 2 | Janela em pacotes |
| 22 | 2 | Tamanho do payload |
| 24 | 4 | CRC32 IEEE |
| 28 | variável | Payload |

CRC calculado sobre cabeçalho + payload, com campo CRC zerado. Não é proteção contra adulteração intencional.

Tipos: START=1, DATA=2, ACK=3, FINISH=4, DONE=5, FAILURE=6, REQUEST=7, RESPONSE=8. Os dois últimos foram adicionados na etapa 2; recompilar ambos os lados para usar o serviço.

1. START, sequência 0: JSON `{size, sha256}`, janela oferecida. O receptor responde ACK 0 com o mínimo entre a janela oferecida e seu limite. Repetir START gera o mesmo ACK, sem recriar o arquivo.
2. DATA: sequência começa em 1. O último payload pode ser menor. O receptor só aceita dados no intervalo permitido, armazena os fora de ordem e escreve a sequência contínua. ACK confirma uma sequência individual. Duplicatas anteriores à base recebem ACK novamente.
3. O emissor mantém a base na menor sequência ainda não confirmada. Não envia além de base + janela. Cada pacote não confirmado tem temporizador; só ele é reenviado ao expirar (Selective Repeat).
4. FINISH: sequência número de pacotes + 1. Só é aceito quando todos os dados foram escritos. Confere SHA-256, sincroniza o arquivo e publica sem sobrescrever outro arquivo. Responde DONE e permanece temporariamente disponível para repetir essa confirmação.
5. FAILURE contém descrição curta. Timeout e limite de tentativas impedem espera infinita. CTRL+C cancela; no receptor, temporários incompletos são removidos.

Arquivo vazio: START seguido de FINISH 1; o hash deve ser o SHA-256 vazio. Endereço remoto + ID de sessão identificam o par; isso ainda não autentica nem permite migração.

Estado do emissor: hash -> negociação -> envio com janela -> término -> concluído/erro.

Estado do receptor: espera -> negociação -> recepção -> validação/publicação -> espera de FINISH repetido -> concluído.

Limitações do transporte: uma sessão por socket, janela fixa negociada, sem autenticação/criptografia/congestionamento embutidos. A janela do ACK não representa espaço livre dinâmico; o buffer é limitado pela janela negociada. A etapa 3 adiciona RTO adaptativo para DATA (em validação); controle e espera final continuam com RTO fixo. O formato dos pacotes não mudou.

## Camada de aplicação (etapa 2)

REQUEST leva JSON com `operation`, `password`, `name` (transferências) e `offset` (listagem). O campo Session contém um ID aleatório do pedido. RESPONSE repete o ID e leva `error`, ou lista de arquivos e próxima página, ou objeto vazio autorizando transferência. Os dois tipos usam o CRC32 do formato RC2U.

Cada pedido autentica por senha simples. O cliente retransmite REQUEST até receber RESPONSE ou atingir o limite. Durante uma transferência, o servidor repete a resposta de autorização se receber novamente o pedido original. O transporte então usa um novo ID de sessão e só aceita o endereço UDP autenticado. Todos os pacotes utilizam a mesma porta do serviço.

O servidor aceita apenas uma operação por vez e mantém cache limitado de pedidos. Ao finalizar, marca o pedido como encerrado para impedir execução duplicada. Os dados e credenciais ainda não são criptografados. Detalhes e limitações em ETAPA2.md.
