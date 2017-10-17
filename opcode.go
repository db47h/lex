package asm

type Opcode int32

const (
	LOAD     Opcode = 0x03 // 000 0011
	LOAD_FP  Opcode = 0x07 // 000 0111
	MISC_MEM Opcode = 0x0F // 000 1111
	OP_IMM   Opcode = 0x13 // 001 0011
	AUIPC    Opcode = 0x17 // 001 0111
	OP_IMM32 Opcode = 0x1B // 001 1011
	STORE    Opcode = 0x23 // 010 0011
	STORE_FP Opcode = 0x27 // 010 0111
	AMO      Opcode = 0x2F // 010 1111
	OP       Opcode = 0x33 // 011 0011
	LUI      Opcode = 0x37 // 011 0111
	OP_32    Opcode = 0x3B // 011 1011
	MADD     Opcode = 0x43 // 100 0011
	MSUB     Opcode = 0x47 // 100 0111
	NMSUB    Opcode = 0x4B // 100 1011
	NMADD    Opcode = 0x4F // 100 1111
	OP_FP    Opcode = 0x53 // 101 0011
	BRANCH   Opcode = 0x63 // 110 0011
	JALR     Opcode = 0x67 // 110 0111
	JAL      Opcode = 0x6F // 110 1111
	SYSTEM   Opcode = 0x73 // 111 0011
)

type Format int

const (
	FmtNone Format = iota // instruction w/o arguments
	FmtRR                 // rd rs1 rs1
	FmtUJ                 // rd #imm20
	FmtRI                 // rd rs1 #imm12
	FmtIR                 // rd #imm12(rs1)
	FmtRI5                // rd rs1 #imm5
	FmtI5I                // rd #imm5 #imm12
	FmtFE                 // FENCE
)

type Instruction struct {
	Name   string
	Mask   Opcode
	Format Format
}

var Instructions = map[string]Instruction{
	// RV32I
	"lui":     {"lui", LUI, FmtUJ},
	"auipc":   {"auipc", AUIPC, FmtUJ},
	"jal":     {"jal", JAL, FmtUJ},
	"jalr":    {"jalr", JALR, FmtRI},
	"beq":     {"beq", 0<<12 | BRANCH, FmtRI},
	"bne":     {"bne", 1<<12 | BRANCH, FmtRI},
	"blt":     {"blt", 4<<12 | BRANCH, FmtRI},
	"bge":     {"bge", 5<<12 | BRANCH, FmtRI},
	"bltu":    {"bltu", 6<<12 | BRANCH, FmtRI},
	"bgeu":    {"bgeu", 7<<12 | BRANCH, FmtRI},
	"lb":      {"lb", 0<<12 | LOAD, FmtIR},
	"lh":      {"lh", 1<<12 | LOAD, FmtIR},
	"lw":      {"lw", 2<<12 | LOAD, FmtIR},
	"lbu":     {"lbu", 4<<12 | LOAD, FmtIR},
	"lhu":     {"lhu", 5<<12 | LOAD, FmtIR},
	"sb":      {"sb", 0<<12 | STORE, FmtIR},
	"sh":      {"sh", 1<<12 | STORE, FmtIR},
	"sw":      {"sw", 2<<12 | STORE, FmtIR},
	"addi":    {"addi", 0<<12 | OP_IMM, FmtRI},
	"slti":    {"slti", 2<<12 | OP_IMM, FmtRI},
	"sltiu":   {"sltiu", 3<<12 | OP_IMM, FmtRI},
	"xori":    {"xori", 4<<12 | OP_IMM, FmtRI},
	"ori":     {"ori", 6<<12 | OP_IMM, FmtRI},
	"andi":    {"andi", 7<<12 | OP_IMM, FmtRI},
	"slli":    {"slli", 1<<12 | OP_IMM, FmtRI5},
	"srli":    {"srli", 5<<12 | OP_IMM, FmtRI5},
	"srai":    {"srai", 1<<30 | 5<<12 | OP_IMM, FmtRI5},
	"add":     {"add", 0<<12 | OP, FmtRR},
	"sub":     {"sub", 1<<30 | 0<<12 | OP, FmtRR},
	"sll":     {"sll", 1<<12 | OP, FmtRR},
	"slt":     {"slt", 2<<12 | OP, FmtRR},
	"sltu":    {"sltu", 3<<12 | OP, FmtRR},
	"xor":     {"xor", 4<<12 | OP, FmtRR},
	"srl":     {"srl", 5<<12 | OP, FmtRR},
	"sra":     {"sra", 1<<30 | 5<<12 | OP, FmtRR},
	"or":      {"or", 6<<12 | OP, FmtRR},
	"and":     {"or", 7<<12 | OP, FmtRR},
	"fence":   {"fence", 0<<12 | MISC_MEM, FmtFE},
	"fence.i": {"fence.i", 1<<12 | MISC_MEM, FmtNone},
	"ecall":   {"ecall", 0<<20 | SYSTEM, FmtNone},
	"ebreak":  {"ebreak", 1<<20 | SYSTEM, FmtNone},
	"csrrw":   {"csrrw", 1<<12 | SYSTEM, FmtRI},
	"csrrs":   {"csrrs", 2<<12 | SYSTEM, FmtRI},
	"csrrc":   {"csrrc", 3<<12 | SYSTEM, FmtRI},
	"csrrwi":  {"csrrwi", 5<<12 | SYSTEM, FmtI5I},
	"csrrsi":  {"csrrsi", 6<<12 | SYSTEM, FmtI5I},
	"csrrci":  {"csrrci", 7<<12 | SYSTEM, FmtI5I},
	// RV32IM
	"mul":    {"mul", 1<<25 | 0<<12 | OP, FmtRR},
	"mulh":   {"mulh", 1<<25 | 1<<12 | OP, FmtRR},
	"nulhsu": {"nulhsu", 1<<25 | 2<<12 | OP, FmtRR},
	"mulu":   {"mulu", 1<<25 | 3<<12 | OP, FmtRR},
	"div":    {"div", 1<<25 | 4<<12 | OP, FmtRR},
	"divu":   {"divu", 1<<25 | 5<<12 | OP, FmtRR},
	"rem":    {"rem", 1<<25 | 6<<12 | OP, FmtRR},
	"remu":   {"remu", 1<<25 | 7<<12 | OP, FmtRR},
	// Privileged instructions
	"wfi": {"wfi", 0x105<<20 | SYSTEM, FmtNone},
}
