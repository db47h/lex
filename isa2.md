# ISA thoughts

## Riscv like ISA

The ISA is identical to RV32IM with a modified binary encoding that is better suited for software emulation.

System instructions are not implemented with the exception of WFI.

### Registers

Register | ABI Name | Description                       | Saver
---------|----------|-----------------------------------|-------
x0       | zero     | Hard-wired zero                   | —
x1       | ra       | Return address                    | Caller
x2       | sp       | Stack pointer                     | Callee
x3       | gp       | Global pointer                    | —
x4       | tp       | Thread pointer                    | —
x5       | t0       | Temporary/alternate link register | Caller
x6–7     | t1–2     | Temporaries                       | Caller
x8       | s0/fp    | Saved register/frame pointer      | Callee
x9       | s1       | Saved register                    | Callee
x10–11   | a0–1     | Function arguments/return values  | Caller
x12–17   | a2–7     | Function arguments                | Caller
x18–27   | s2–11    | Saved registers                   | Callee
x28–31   | t3–6     | Temporaries                       | Caller

### Instruction formats

In order to limit bit twiddling while decoding instructions in software, instruction formats have been simplified compared to Risc-V:

- Stores use the I format (SW rd #imm12(rs1)), this allows to get the #imm12 value directly.
- Branches use the I fromat for the same reason.
- Risc-V's U and J formats have been merged into a single UJ format.

```txt
   3                   2                   1
 1 0 9 8 7 6 5 4 3 2 1 0 9 8 7 6 5 4 3 2 1 0 9 8 7 6 5 4 3 2 1 0
|    func7    |   rs2   |   rs1   |func3|   rd    |   opcode    | R
|    #imm12             |   rs1   |func3|   rd    |   opcode    | I
|          #imm20                       |   rd    |   opcode    | UJ
```

Some instructions interpret the #imm12 or rs1 field differently. The alternative format for these are listed below.

```txt
   3                   2                   1
 1 0 9 8 7 6 5 4 3 2 1 0 9 8 7 6 5 4 3 2 1 0 9 8 7 6 5 4 3 2 1 0
|    func7    |  #imm5  |   rs1   |func3|   rd    |   opcode    | I' (Shifts with immediate argument)
|    #imm12             |  #imm5  |func3|   rd    |   opcode    | I" (CSRxxI)
```

### Opcode map

The two low order bits of the opcode field are always 11.

Opcode  | Name
--------|-------
0000011 | LOAD
0000111 | LOAD-FP
0001011 | *custom-0*
0001111 | MISC-MEM
0010011 | OP-IMM
0010111 | AUIPC
0011011 | OP-IMM32
0011111 | *48b instruction*
0100011 | STORE
0100111 | STORE-FP
0101011 | *custom-1*
0101111 | AMO
0110011 | OP
0110111 | LUI
0111011 | OP-32
0111111 | *64b instruction*
1000011 | MADD
1000111 | MSUB
1001011 | NMSUB
1001111 | NMADD
1010011 | OP-FP
1010111 | *reserved*
1011011 | *custom-2/rv128*
1011111 | *48b instruction*
1100011 | BRANCH
1100111 | JALR
1101011 | *reserved*
1101111 | JAL
1110011 | SYSTEM
1110111 | *reserved*
1111011 | *custom-3/rv128*
1111111 | *≥80b instruction*

### Instruction set list

Opcode   | Format | Func3 | Func7 | Asm    | Args             | Result                                | Comments
---------|--------|-------|-------|--------|------------------|---------------------------------------|----------
LUI      | UJ     |       |       | LUI    | rd #imm20        | rd = #imm20 << 12                     |
AUIPC    | UJ     |       |       | AUIPC  | rd #imm20        | rd = pc + (#imm20 << 12)              |
JAL      | UJ     |       |       | JAL    | rd #imm20        | rd = pc; pc += #imm21 << 2            |
JALR     | I      |  000  |       | JALR   | rd rs1 #imm16    | rd = pc; pc = (rs1 + #imm16 << 2)     |
BRANCH   | I      |  000  |       | BEQ    | rd rs1 #imm16    | if (rd == rs1) pc += (#imm16 << 2)    |
BRANCH   | I      |  001  |       | BNE    | rd rs1 #imm16    | if (rd != rs1) pc += (#imm16 << 2)    |
BRANCH   | I      |  100  |       | BLT    | rd rs1 #imm16    | if (rd < rs1) pc += (#imm16 << 2)     |
BRANCH   | I      |  101  |       | BGE    | rd rs1 #imm16    | if (rd >= rs1) pc += (#imm16 << 2)    |
BRANCH   | I      |  110  |       | BLTU   | rd rs1 #imm16    | if (rd < rs1) pc += (#imm16 << 2)     |
BRANCH   | I      |  111  |       | BGEU   | rd rs1 #imm16    | if (rd >= rs1) pc += (#imm16 << 2)    |
LOAD     | I      |  000  |       | LB     | rd #imm16(rs1)   | rd = *(rs1 + #imm16)                  | byte op, sign extend value
LOAD     | I      |  001  |       | LH     | rd #imm16(rs1)   | rd = *(rs1 + #imm16)                  | short op, sign extend value
LOAD     | I      |  010  |       | LW     | rd #imm16(rs1)   | rd = *(rs1 + #imm16)                  | word op, sign extend value
LOAD     | I      |  100  |       | LBU    | rd #imm16(rs1)   | rd = *(rs1 + #imm16)                  | byte op, unsigned extend value
LOAD     | I      |  101  |       | LHU    | rd #imm16(rs1)   | rd = *(rs1 + #imm16)                  | short op, unsigned extend value
LOAD     | I      |  000  |       | SB     | rd #imm16(rs1)   | (*rs1 + #imm16) = rd                  | byte op
STORE    | I      |  001  |       | SH     | rd #imm16(rs1)   | (*rs1 + #imm16) = rd                  | short op
STORE    | I      |  010  |       | SW     | rd #imm16(rs1)   | (*rs1 + #imm16) = rd                  | word op
OP-IMM   | I      |  000  |       | ADDI   | rd rs1 #imm16    | rd = rs1 + #imm16                     |
OP-IMM   | I      |  010  |       | SLTI   | rd rs1 #imm16    | rd = rs1 < #imm16 ? 1 : 0             | signed
OP-IMM   | I      |  011  |       | SLTIU  | rd rs1 #imm16    | rd = rs1 < #imm16 ? 1 : 0             | unsigned
OP-IMM   | I      |  100  |       | XORI   | rd rs1 #imm16    | rd = rs1 ^ #imm16                     |
OP-IMM   | I      |  110  |       | ORI    | rd rs1 #imm16    | rd = rs1 \| #imm16                    |
OP-IMM   | I      |  111  |       | ANDI   | rd rs1 #imm16    | rd = rs1 & #imm16                     |
OP-IMM   | I'     |  001  |0000000| SLLI   | rd rs1 #imm5     | rd = rs1 << #imm5                     |
OP-IMM   | I'     |  101  |0000000| SRLI   | rd rs1 #imm5     | rd = rs1 >> #imm5                     | unsigned
OP-IMM   | I'     |  101  |0100000| SRAI   | rd rs1 #imm5     | rd = rs1 >> #imm5                     | signed
OP       | R      |  000  |0000000| ADD    | rd rs1 rs2       | rd = rs1 + rs2                        |
OP       | R      |  000  |0100000| SUB    | rd rs1 rs2       | rd = rs1 - rs2                        |
OP       | R      |  001  |0000000| SLL    | rd rs1 rs2       | rd = rs1 << rs2                       |
OP       | R      |  010  |0000000| SLT    | rd rs1 rs2       | rd = rs1 < rs2 ? 1 : 0                | signed
OP       | R      |  011  |0000000| SLTU   | rd rs1 rs2       | rd = rs1 < rs2 ? 1 : 0                | unsigned
OP       | R      |  100  |0000000| XOR    | rd rs1 rs2       | rd = rs1 ^ rs2                        |
OP       | R      |  101  |0000000| SRL    | rd rs1 rs2       | rd = rs1 >> rs2                       | unsigned
OP       | R      |  101  |0100000| SRA    | rd rs1 rs2       | rd = rs1 >> rs2                       | signed
OP       | R      |  110  |0000000| OR     | rd rs1 rs2       | rd = rs1 \| rs2                       |
OP       | R      |  111  |0000000| AND    | rd rs1 rs2       | rd = rs1 & rs2                        |
MISC-MEM | -      |  000  |       | FENCE  |                  |                                       | NOT IMPLEMENTED
MISC-MEM | -      |  001  |       | FENCE.I|                  |                                       | NOT IMPLEMENTED
SYSTEM   | -      |  000  |0000000| ECALL  |                  |                                       | NOT IMPLEMENTED
SYSTEM   | -      |  000  |0000000| EBREAK |                  |                                       | NOT IMPLEMENTED
SYSTEM   | -      |  000  |0001000| WFI    |                  | wait_for_interrupt()                  | encoded as 0x10500073
SYSTEM   | I      |  001  |       | CSRRW  | rd rs1 #imm12    | rd = *csr[#imm6]; *csr[#imm6] = rs1   |
SYSTEM   | I      |  010  |       | CSRRS  | rd rs1 #imm12    | rd = *csr[#imm6]; *csr[#imm6] \|= rs1 |
SYSTEM   | I      |  011  |       | CSRRC  | rd rs1 #imm12    | rd = *csr[#imm6]; *csr[#imm6] &= !rs1 |
SYSTEM   | I"     |  101  |       | CSRRWI | rd #imm5 #imm12  | rd = *csr[#imm6]; *csr[#imm6] = #imm5   |
SYSTEM   | I"     |  110  |       | CSRRSI | rd #imm5 #imm12  | rd = *csr[#imm6]; *csr[#imm6] \|= #imm5 |
SYSTEM   | I"     |  111  |       | CSRRCI | rd #imm5 #imm12  | rd = *csr[#imm6]; *csr[#imm6] &= !#imm5 |
OP       | R      |  000  |0000001| MUL    | rd rs1 rs2       | rd = rs1 * rs2                        |
OP       | R      |  001  |0000001| MULH   | rd rs1 rs2       | rd = (rs1 * rs2) >> 32                | signed\*signed
OP       | R      |  010  |0000001| MULHSU | rd rs1 rs2       | rd = (rs1 * rs2) >> 32                | signed\*unsigned
OP       | R      |  011  |0000001| MULHU  | rd rs1 rs2       | rd = (rs1 * rs2) >> 32                | unsigned\*unsigned
OP       | R      |  100  |0000001| DIV    | rd rs1 rs2       | rd = rs1 / rs2                        | signed
OP       | R      |  101  |0000001| DIVU   | rd rs1 rs2       | rd = rs1 / rs2                        | unsigned
OP       | R      |  110  |0000001| REM    | rd rs1 rs2       | rd = rs1 % rs2                        | signed
OP       | R      |  111  |0000001| REMU   | rd rs1 rs2       | rd = rs1 % rs2                        | unsigned

### Pseudo instructions

Pseudo instruction | Base instruction
-------------------|-------------------
li rd #imm32       | lui rd (#imm32 + 0x8000) >> 16 ; addi rd x0 (#imm32 - ((#imm32 + 0x8000) & 0xFFFF))
la rd symbol       | auipc rd (symbol + 0x8000) >> 16 ; addi rd x0 (symbol - ((symbol + 0x8000) & 0xFFFF))
j symbol           | jal x0 symbol
jr rs              | jalr x0 rs 0
call symbol        | auipc x6 (symbol + 0x8000) >> 16 ; jalr x1 x6 (symbol - ((symbol + 0x8000) & 0xFFFF))

## A 16 bits version

Instruction format:

```txt
           1
 5 4 3 2 1 0 9 8 7 6 5 4 3 2 1 0
|  rs1  | #imm4 |  rd   |       | RI
|  rs1  |  rs2  |  rd   |       | RR
| #imm8         |  rd   |       | I
|  rs1  |0|#fun3|  rd   |1 1 1 1| R
|  rs1  |1|#imm3|  rd   |1 1 1 1| RI3
```

Registers:

Register | ABI name | Description
---------|----------|------------
x0       | zero     | hard-wired zero
x1       | ra       |
x2       | sp       |
x3       | t0       |
x4       | t1       |
x5       | t2       |
x6       | t3       |
x7       | rt       | Temp register used by the assembler
x8       | s0       |
x9       | s1       |
x10      | s2       |
x11      | s3       |
x12      | a0       |
x13      | a1       |
x14      | a2       |
x15      | a3       |

Format | Op |special encoding| Asm   | Args                               | Comments
-------|----|--------|-------|------------------------------------|----------
RR     |  0 |rsX!=x0 | AND   | rd = rs1 & rs2                     |
RR     |  0 | rs2=x0 | -     |                                    | FREE for OP rd rs1
RR     |  1 |rsX!=x0 | OR    | rd = rs1 | rs2                     |
RR     |  1 | rs2=x0 | -     |                                    | FREE for OP rd rs1
RR     |  2 |rsX!=x0 | XOR   | rd = rs1 ^ rs2                     |
RR     |  2 | rs2=x0 | NOT   | rd = ~rs1                          | Encoded as XOR rd rs1 zero.
RR     |  3 |rsX!=x0 | SRL   | rd = rs2<0? rs1 << rs2 : rs1 >> rs2| Unsigned logical shift. if rs2 < 0, shift left
RR     |  3 | rs2=x0 | -     |                                    | FREE for OP rd rs1
RR     |  4 |rsX!=x0 | SRA   | rd = rs2<0? rs1 << rs2 : rs1 >> rs2| Arithmetic shift. If rs2 < 0, shift left (pads lsb with rs1[0])
RR     |  4 | rs2=x0 | SWP   | rd = rs1<<8 | rs1 >> 8             | Swap high an low bytes. Encoded as "SHA rd rs1 zero"
RR     |  5 |        | ADD   | rd = rs1 + rs2                     |
RR     |  6 |        | SUB   | rd = rs1 - rs2                     |
I      |  7 |        | JAL   | rd = pc + 2; pc = pc + #imm8 << 1  |
I      |  8 | rd!=x0 | LI    | rd = #imm8                         | Sign extended
I      |  8 | rd=x0  | -     |                                    | FREE for OP #imm8 (ECALL, I/O, ...) or OP rs1 rs2
I      |  9 | rd!=x0 | LUI   | rd = #imm8 << 8 | rd & 0xFF        | \*
I      |  9 | rd=x0  | -     |                                    | FREE for OP #imm8 (WFI, EBREAK). or OP rs1 rs2 (OPEN/CLOSE channel?)
I      | 10 | rd!=x0 | AUIPC | rd = %hi(pc + #imm8)<<8 | rd & 255 | \*
I      | 10 | rd=x0  | -     |                                    | FREE for OP #imm8
RI     | 11 |        | LW    | rd = rs1[#imm4]                    | Load 16 bits word **
RI     | 12 |        | SW    | rs1[#imm4] = rd                    | Store 16 bits word
RI     | 13 |        | LB    | rd = rs1[#imm4]                    | Load byte, sign extended.
RI     | 14 |        | SB    | rs1[#imm4] = rd                    | Store byte
R      | 15 |#fun3=0 | JALR  | rd = pc + 2; pc = rs1              |
R      | 15 |        | IFxxx | if (rd comp_op rs1) pc+=2 else pc+=4| Execute next insn only if cmp_op yields true. cmp_op encoded in #imm3 field (EQ, NE, LT, GE, LTU, GEU).
RI3    | 15 |        | ADDI  | rd = rs1 + #imm3                   |

\* Only changes upper 8 bits. Since we don't have "addi", this allows loading 16 bits ints with an li, lui sequence. Loading integers outside of the range [-128, 127] requires a two instruction sequence: li, followed by lui or auipc:

    ; hi = ((unsigned)#imm16) >> 8
    ; lo = #imm16 & 0xFF
    li rd #lo
    lui/auipc rd #hi

** load/stores on #imm4[zero] can be used to access 16 8 bits CSRs. #imm4 treated as positive offset. rd=x0 is legal since reading a CSR or mapped I/O address may have side effects even if we don't care about the result.

IFxxx encoding:

\#fun3 field | Asm    | Args
-------------|--------|--------------
0            | JALR   | rd = pc + 2; pc = rs1
1            | IFEQ   | rd == rs1 ? ...
2            | IFNE   | rd != rs1 ? ...
3            | IFLT   | rd < rs1 ? ...
4            | IFGE   | rd >= rs1 ?
5            | IFLTU  | (unsigned)rd < (unsigned)rs1 ?
6            | IFGEU  | (unsigned)rd >= (unsigned)rs1 ?
7            | -      | FREE (bit test ?)

Pseudo instructions:

Pseudo instruction | Base instruction
-------------------|-----------------
nop                | add zero zero zero
li rd #imm16       | li rd %lo(#imm16) ; lui rd %hi(#imm16)
la rd symbol       | li rd %lo(symbol_offset) ; auipc rd %hi(symbol_offset)
mv rd rs1          | add rd rs1 zero
ifgt rd rs1        | iflt rs1 rd
ifgtu rd rs1       | ifltu rs1 rd
ifle rd rs1        | ifge rs1 rd
ifleu rd rs1       | ifgeu rs1 rd

Have the assembler do the following conversions (assembler macros would work):

```txt
strcmp:
1:
  lb t0 (a0)
  lb t1 (a1)
  bne t0 t1 fail
  beq t0 zero pass
  addi a0 a0 1
  addi a1 a1 1
  jal zero 1b
pass:
  addi a0 zero 0
  jalr zero ra
fail:
  addi a0 zero 1
  jalr zero ra
```

to :

```txt
strcmp:
1:
  lb t0 (a0)
  lb t1 (a1)
  ifne t0 t1   ; was: bne t0 t1 fail
  jal zero fail
  ifeq t0 zero ; was: beq t0 zero pass
  jal zero pass
  addi a0 a0 1
  addi a1 a1 1
  jal zero 1b
done:
  addi a0 zero 0
  jalr zero ra
fail:
  addi a0 zero 1
  jalr zero ra
```

Typical function :

```asm
func:
  ; prologue
  li t0 10       ; make room for saved regs
  sub sp sp t0   ; ...
  sw s0 0(sp)    ; save callee-saved registers
  sw s1 2(sp)
  sw s2 4(sp)
  sw s3 6(sp)
  sw ra 8(sp)   ; save ra
  add s3 sp t0  ; get ptr to args on stack (t4 = sp value when entering the function)
  li t0 16      ; make room for some temp storage on stack
  sub sp sp t0  ; ...

  ; access stack args with lw/sw rd #offset[s3]
  ; access local vars with lw/sw rd #offset[sp]

  ; epilogue
  li t0 16
  add sp sp t0 ; get saved registers address
  lw s0 0(sp)
  lw s1 2(sp)
  lw s2 4(sp)
  lw s3 6(sp)
  lw ra 8(sp)
  add sp sp t0 ; restore sp
  jalr zero ra 0
```

With no args on stack or local storage:

```asm
func:
  ; prologue
  li t0 10
  sub sp sp t0
  sw s0 0(sp)    ; save callee-saved registers
  sw s1 2(sp)
  sw s2 4(sp)
  sw s3 6(sp)
  sw ra 8(sp)   ; save ra

  ; do things

  ; epilogue
  lw s0 0(sp)
  lw s1 2(sp)
  lw s2 4(sp)
  lw s3 6(sp)
  lw ra 8(sp)
  li t0 10
  add sp sp t0
  jalr zero ra 0
```

```asm
sext:
  swp a0 a0
  li t0 8
  ash a0 a0 t0
  jalr zero ra 0

sext2:
  lui a0 0        ; clear upper bits
  li t0 127
  ifltu t0 a0
    lui a0 0xff   ; if 127 < (unsigned)a0 a0 |= 0xFF00
  jalr zero ra 0
```

### if else

```asm
if_else:
  ; if (a0 = a1) return 0 else return 1;
  ifeq a0 a1
  j cond_true
cond_false:
  li a0 0
  j if_end
cond_true:
  li a1 0
if_end:
  jr ra
```

### overflow checks

```asm
unsigned_add:
  add a0 a0 a1
  ifltu a0 a1  ; !
  j overflow   ; !
  li a1 0
  jr ra
overflow:
  li a1 1
  jr ra

signed_addi:
  addi a1 a0 0 ; save a0
  addi a0 a0 #some_positive_immediate
  iflt a0 a1  ; !
  j overflow  ; !
  li a1 0
  jr ra

signed_add:
  mv a2 a0
  add a0 a0 a1
  li t0 15     ; !
  sra t0 a1 t0 ; ! t0 = a1 < 0 ? -1 : 0
  iflt a0 a2   ; !
  not t0       ; !
  ifne t0      ; !
  j overflow   ; !
  li a1 0
  jr ra
```
