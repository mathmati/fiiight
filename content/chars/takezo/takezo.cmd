;-| Super Motions |--------------------------------------------------------
;  ---------------
[Command]
name = "QCFQCF_z"
command = ~D, F, D, F, z
time = 30

[Command]
name = "QCFQCF_y"
command = ~D, F, D, F, y
time = 30

[Command]
name = "QCFQCF_x"
command = ~D, F, D, F, x
time = 30

[Command]
name = "QCFQCF_a"
command = ~D, F, D, F, a
time = 30

[Command]
name = "QCFQCFQCF_z"
command = ~D, F, D, F, D, F, z
time = 40

[Command]
name = "QCFQCFQCF_y"
command = ~D, F, D, F, D, F, y
time = 40

[Command]
name = "QCFQCFQCF_x"
command = ~D, F, D, F, D, F, x
time = 40

;-| Special Motions |------------------------------------------------------
;-----------------
; QCF
[Command]
name = "QCF_a"
command = ~D, F, a
time = 10

[Command]
name = "QCF_b"
command = ~D, F, b
time = 10

[Command]
name = "QCF_c"
command = ~D, F, c
time = 10

[Command]
name = "QCF_x"
command = ~D, F, x
time = 10

[Command]
name = "QCF_y"
command = ~D, F, y
time = 10

[Command]
name = "QCF_z"
command = ~D, F, z
time = 10

;-----------------
; QCB_FWD
[Command]
name = "QCB_FWD_z"
command = ~DB, B, F, z
time = 15

[Command]
name = "QCB_FWD_y"
command = ~DB, B, F, y
time = 15

[Command]
name = "QCB_FWD_x"
command = ~DB, B, F, x
time = 15

;-----------------
; QCB
[Command]
name = "QCB_a"
command = ~D, B, a
time = 15

[Command]
name = "QCB_b"
command = ~D, B, b
time = 15

[Command]
name = "QCB_c"
command = ~D, B, c
time = 15

[Command]
name = "QCB_x"
command = ~D, B, x
time = 15

[Command]
name = "QCB_y"
command = ~D, B, y
time = 15

[Command]
name = "QCB_z"
command = ~D, B, z
time = 15

;-----------------
; Uppercut
[Command]
name = "uppercut_x"
command = ~F, D, DF, x
time = 20

[Command]
name = "uppercut_y"
command = ~F, D, DF, y
time = 20

[Command]
name = "uppercut_z"
command = ~F, D, DF, z
time = 20

;-----------------
; HCF
[Command]
name = "HCF_a"
command = ~DB, D, DF, a
time = 10

[Command]
name = "HCF_b"
command = ~DB, D,D F, b
time = 10

[Command]
name = "HCF_c"
command = ~DB, D, DF, c
time = 10

[Command]
name = "HCF_x"
command = ~DB, D, DF, x
time = 10

[Command]
name = "HCF_y"
command = ~DB, D, DF, y
time = 10

[Command]
name = "HCF_z"
command = ~DB, D, DF, z
time = 10


;-| Double Tap |-----------------------------------------------------------
[Command]
name = "FF";Required (do not remove)
command = F, F
time = 12

[Command]
name = "BB";Required (do not remove)
command = B, B
time = 12

;-| 2/3 Button Combination |-----------------------------------------------
[Command]
name = "recovery";Required (do not remove)
command = z
time = 1

[Command]
name = "xy"
command = x+y
time = 1

[Command]
name = "xyz"
command = x+y+z
time = 1

[Command]
name = "ax"
command = a+x
time = 1

[Command]
name = "ab"
command = a+b
time = 1

[Command]
name = "juggle"
command = y+z
time = 1

;-| Dir + Button |---------------------------------------------------------
[Command]
name = "fwd_a"
command = /F,a
time = 1

[Command]
name = "fwd_b"
command = /F,b
time = 1

[Command]
name = "fwd_c"
command = /F,c
time = 1

[Command]
name = "fwd_x"
command = /F,x
time = 1

[Command]
name = "fwd_y"
command = /F,y
time = 1

[Command]
name = "fwd_z"
command = /F,z
time = 1

[Command]
name = "back_a"
command = /B,a
time = 1

[Command]
name = "back_b"
command = /B,b
time = 1

[Command]
name = "back_z"
command = /B,z
time = 1

[Command]
name = "back_c"
command = /B,c
time = 1

[Command]
name = "down_a"
command = /$D,a
time = 1

[Command]
name = "down_b"
command = /$D,b
time = 1

[Command]
name = "down_c"
command = /$D,c
time = 1

[Command]
name = "fwd_ab"
command = /F, a+b
time = 1

[Command]
name = "back_ab"
command = /B, a+b
time = 1

;-| Single Button |---------------------------------------------------------
[Command]
name = "a"
command = a
time = 1

[Command]
name = "b"
command = b
time = 1

[Command]
name = "x"
command = x
time = 1

[Command]
name = "y"
command = y
time = 1

[Command]
name = "z"
command = z
time = 1

[Command]
name = "start"
command = s
time = 1

;-| Hold Dir |--------------------------------------------------------------
[Command]
name = "holdfwd";Required (do not remove)
command = /$F
time = 1

[Command]
name = "holdback";Required (do not remove)
command = /$B
time = 1

[Command]
name = "holdup";Required (do not remove)
command = /$U
time = 1

[Command]
name = "holddown";Required (do not remove)
command = /$D
time = 1

[Command]
name = "charge"
command = /$x+y+z
time = 1

;-| AI |--------------------------------------------------------------
;CPU_AI
[Command]
name = "CPU_AI_Z"
command = ~U, UF, U, UB, U, UF, U, UB, U, UF, U, UB, U, UF, U, UB, U, z
time = 1

[Command]
name = "CPU_AI_Y"
command = ~U, UF, U, UB, U, UF, U, UB, U, UF, U, UB, U, UF, U, UB, U, y
time = 1

[Command]
name = "CPU_AI_X"
command = ~U, UF, U, UB, U, UF, U, UB, U, UF, U, UB, U, UF, U, UB, U, x
time = 1

[Command]
name = "CPU_AI_A"
command = ~U, UF, U, UB, U, UF, U, UB, U, UF, U, UB, U, UF, U, UB, U, a
time = 1

[Command]
name = "CPU_AI_B"
command = ~U, UF, U, UB, U, UF, U, UB, U, UF, U, UB, U, UF, U, UB, U, y
time = 1

[Command]
name = "CPU_AI_C"
command = ~U, UF, U, UB, U, UF, U, UB, U, UF, U, UB, U, UF, U, UB, U, c
time = 1

[Command]
name = "CPU_AI_AZ"
command = ~U, UF, U, UB, U, UF, U, UB, U, UF, U, UB, U, UF, U, UB, U, a
time = 1

[Command]
name = "CPU_AI_AY"
command = ~U, UF, U, UB, U, UF, U, UB, U, UF, U, UB, U, UF, U, UB, U, b
time = 1

[Command]
name = "CPU_AI_AX"
command = ~U, UF, U, UB, U, UF, U, UB, U, UF, U, UB, U, UF, U, UB, U, c
time = 1

[Command]
name = "CPU_AI_BA"
command = ~U, UF, U, UB, U, UF, U, UB, U, UF, U, UB, U, UF, U, UB, U, x
time = 1

[Command]
name = "CPU_AI_BC"
command = ~U, UF, U, UB, U, UF, U, UB, U, UF, U, UB, U, UF, U, UB, U, y
time = 1

[Command]
name = "CPU_AI_ABC"
command = ~U, UF, U, UB, U, UF, U, UB, U, UF, U, UB, U, UF, U, UB, U, z
time = 1

;===========================================================================
;===========================================================================

[Statedef -1]

;===========================================================================
;---------------------------------------------------------------------------
;RunFwd
[State -1]
type = ChangeState
value = 100
trigger1 = command = "FF"
trigger1 = statetype = S
trigger1 = ctrl = 1

;---------------------------------------------------------------------------
;RunBack
[State -1]
type = ChangeState
value = 105
trigger1 = command = "BB"
trigger1 = statetype = S
trigger1 = ctrl = 1

;====================================SUPERS=================================
;---------------------------------------------------------------------------
;SUPER#5:GUARDIAN_FORCE_MUSASHI
[State -1]
type = ChangeState
value = 4500
triggerall = command = "QCFQCFQCF_z"
triggerall = var(50) = 2
triggerall = life <= 200
trigger1 = statetype = S
trigger1 = ctrl = 1
trigger2 = stateno = 220
trigger2 = movecontact = 1
trigger3 = stateno = 200
trigger3 = movecontact = 1
trigger4 = stateno = 400
trigger4 = movecontact = 1
trigger5 = stateno = 1900 && time > 7
trigger6 = stateno = 6740 && movecontact = 1

;SUPER#4:BOWLING
[State -1]
type = ChangeState
value = 4300
triggerall = command = "QCFQCFQCF_y"
triggerall = var(50) = 2
trigger1 = statetype = S
trigger1 = ctrl = 1
trigger2 = stateno = 220
trigger2 = movecontact = 1
trigger3 = stateno = 200
trigger3 = movecontact = 1
trigger4 = stateno = 400
trigger4 = movecontact = 1
trigger5 = stateno = 1900 && time > 7
trigger6 = stateno = 6740 && movecontact = 1

;SUPER#1:EDGE_OF_MASAMUNE
[State -1]
type = ChangeState
value = 4000
triggerall = command = "QCFQCF_x"
triggerall = var(50) = 2
trigger1 = statetype = S
trigger1 = ctrl = 1
trigger2 = stateno = 220
trigger2 = movecontact = 1
trigger3 = stateno = 200
trigger3 = movecontact = 1
trigger4 = stateno = 400
trigger4 = movecontact = 1
trigger5 = stateno = 1900 && time > 7
trigger6 = stateno = 6740 && movecontact = 1

;SUPER#2:DRAGON_FURY
[State -1]
type = ChangeState
value = 4100
triggerall = command = "QCFQCF_y"
triggerall = var(50) = 2
trigger1 = statetype = S
trigger1 = ctrl = 1
trigger2 = stateno = 220
trigger2 = movecontact = 1
trigger3 = stateno = 200
trigger3 = movecontact = 1
trigger4 = stateno = 400
trigger4 = movecontact = 1
trigger5 = stateno = 1900 && time > 7
trigger6 = stateno = 6740 && movecontact = 1

;SUPER#3:DRAGON_FANGS_MAYHEM
[State -1]
type = ChangeState
value = 4200
triggerall = command = "QCFQCF_z"
triggerall = var(50) = 2
trigger1 = statetype = S
trigger1 = ctrl = 1
trigger2 = stateno = 220
trigger2 = movecontact = 1
trigger3 = stateno = 200
trigger3 = movecontact = 1
trigger4 = stateno = 400
trigger4 = movecontact = 1
trigger5 = stateno = 1900 && time > 7
trigger6 = stateno = 6740 && movecontact = 1

;SUPER#5:GUARDIAN_FORCE_MUSASHI_AI
[State -1]
type = ChangeState
value = 4500
triggerall = random < 300 && movehit = 1 && var(50) = 2 && var(40) = 1
triggerall = life <= 200
trigger1 = stateno = 220
trigger2 = stateno = 200
trigger3 = stateno = 400
trigger4 = stateno = 6740

;SUPER#4:BOWLING_AI
[State -1]
type = ChangeState
value = 4300
triggerall = random < 300 && movehit = 1 && var(50) = 2 && var(40) = 1
trigger1 = stateno = 220
trigger2 = stateno = 200
trigger3 = stateno = 400
trigger4 = stateno = 6740

;SUPER#1:EDGE_OF_MASAMUNE_AI
[State -1]
type = ChangeState
value = 4000
triggerall = random < 300 && movehit = 1 && var(50) = 2 && var(40) = 1
trigger1 = stateno = 220
trigger2 = stateno = 200
trigger3 = stateno = 400
trigger4 = stateno = 6740

;SUPER#2:DRAGON_FURY_AI
[State -1]
type = ChangeState
value = 4100
triggerall = random < 300 && movehit = 1 && var(50) = 2 && var(40) = 1
trigger1 = stateno = 220
trigger2 = stateno = 200
trigger3 = stateno = 400
trigger4 = stateno = 6740

;SUPER#3:DRAGON_FANGS_MAYHEM_AI
[State -1]
type = ChangeState
value = 4200
triggerall = random < 300 && movehit = 1 && var(50) = 2 && var(40) = 1
trigger1 = stateno = 220
trigger2 = stateno = 200
trigger3 = stateno = 400
trigger4 = stateno = 6740

;====================================SPECIALS===============================
;---------------------------------------------------------------------------
;BERSERKER
[State -1]
type = ChangeState
value = 1700
triggerall = command = "xyz"
triggerall = var(50) != 2
trigger1 = statetype = S
trigger1 = ctrl = 1

;---------------------------------------------------------------------------
;DRAGON_AURA_BLAST
[State -1]
type = ChangeState
value = 310
triggerall = command = "ab"
triggerall = command != "holddown"
trigger1 = statetype = S
trigger1 = ctrl = 1

;---------------------------------------------------------------------------
;EVADE
[State -1]
type = ChangeState
value = 1900
triggerall = command = "xy"
triggerall = command != "holddown"
trigger1 = statetype = S
trigger1 = ctrl = 1

;---------------------------------------------------------------------------
;UPPERCUT_STRONG
[State -1]
type = ChangeState
value = 1599
triggerall = command = "uppercut_z"
trigger1 = statetype = S
trigger1 = ctrl = 1
trigger2 = stateno = 220
trigger2 = movecontact = 1
trigger3 = stateno = 420
trigger3 = movecontact = 1
trigger4 = stateno = 200
trigger4 = movecontact = 1
trigger5 = stateno = 400
trigger5 = movecontact = 1
trigger6 = stateno = 1900 && time > 7
trigger7 = stateno = 6740 && movecontact = 1

;UPPERCUT_MEDIUM
[State -1]
type = ChangeState
value = 1799
triggerall = command = "uppercut_y"
trigger1 = statetype = S
trigger1 = ctrl = 1
trigger2 = stateno = 220
trigger2 = movecontact = 1
trigger3 = stateno = 420
trigger3 = movecontact = 1
trigger4 = stateno = 200
trigger4 = movecontact = 1
trigger5 = stateno = 400
trigger5 = movecontact = 1
trigger6 = stateno = 1900 && time > 7
trigger7 = stateno = 6740 && movecontact = 1

;UPPERCUT_LIGHT_FOLLOW_UP
[State -1]
type = ChangeState
value = 1851
triggerall = command = "uppercut_x"
trigger1 = stateno = 1010
trigger1 = movecontact = 1

;UPPERCUT_LIGHT
[State -1]
type = ChangeState
value = 1849
triggerall = command = "uppercut_x"
trigger1 = statetype = S
trigger1 = ctrl = 1
trigger2 = stateno = 220
trigger2 = movecontact = 1
trigger3 = stateno = 420
trigger3 = movecontact = 1
trigger4 = stateno = 200
trigger4 = movecontact = 1
trigger5 = stateno = 400
trigger5 = movecontact = 1
trigger6 = stateno = 1900 && time > 7
trigger7 = stateno = 6740 && movecontact = 1

;---------------------------------------------------------------------------
;UPPERCUT_STRONG_FULLPOWER_NEO_BLADE_AIR_COMBO
[State -1]
type = ChangeState
value = 1002
triggerall = command = "QCF_z"
triggerall = power >= 3000
triggerall = statetype = A
trigger1 = stateno = 1600 && movecontact = 1 && time >= 8
trigger2 = stateno = 1601 && p2movetype = H

;NEO_BLADE_OF_THOUSAND_DEMONS_STRONG
[State -1]
type = ChangeState
value = 999
triggerall = command = "QCF_z"
trigger1 = statetype = S
trigger1 = ctrl = 1
trigger2 = stateno = 220
trigger2 = movecontact = 1
trigger3 = stateno = 420
trigger3 = movecontact = 1
trigger4 = stateno = 200
trigger4 = movecontact = 1
trigger5 = stateno = 400
trigger5 = movecontact = 1
trigger6 = stateno = 1900 && time > 7
trigger7 = stateno = 6740 && movecontact = 1

;BLADE_OF_FIVEHUNDRED_DEMONS_MEDIUM
[State -1]
type = ChangeState
value = 1004
triggerall = command = "QCF_y"
trigger1 = statetype = S
trigger1 = ctrl = 1
trigger2 = stateno = 220
trigger2 = movecontact = 1
trigger3 = stateno = 420
trigger3 = movecontact = 1
trigger4 = stateno = 200
trigger4 = movecontact = 1
trigger5 = stateno = 400
trigger5 = movecontact = 1
trigger6 = stateno = 1900 && time > 7
trigger7 = stateno = 6740 && movecontact = 1

;TAURUS_STRIKE
[State -1]
type = ChangeState
value = 1009
triggerall = command = "QCF_x"
trigger1 = statetype = S
trigger1 = ctrl = 1
trigger2 = stateno = 220
trigger2 = movecontact = 1
trigger3 = stateno = 420
trigger3 = movecontact = 1
trigger4 = stateno = 200
trigger4 = movecontact = 1
trigger5 = stateno = 400
trigger5 = movecontact = 1
trigger6 = stateno = 1900 && time > 7
trigger7 = stateno = 6740 && movecontact = 1

;---------------------------------------------------------------------------
;THROW
[State -1]
type = ChangeState
value = 900
triggerall = statetype = S
triggerall = ctrl = 1
triggerall = p2bodydist X < 5 ;Near P2
trigger1 = command = "fwd_z";p2 stand
trigger1 = stateno != 100    ;Not running
trigger1 = p2statetype = S
trigger1 = p2movetype != H
trigger2 = command = "fwd_z";p2 crouch
trigger2 = stateno != 100    ;Not running
trigger2 = p2statetype = C
trigger2 = p2movetype != H
trigger3 = command = "back_z";p2 stand
trigger3 = p2statetype = S
trigger3 = p2movetype != H
trigger4 = command = "back_z";p2 crouch
trigger4 = p2statetype = C
trigger4 = p2movetype != H

;---------------------------------------------------------------------------
;BLACK_SPADE_STRONG
[State -1]
type = ChangeState
value = 1105
triggerall = command = "QCB_z" && numprojid(1100) = 0
trigger1 = statetype = S && ctrl = 1
trigger2 = stateno = 1900 && time > 7
trigger3 = stateno = 6740 && movecontact = 1

;NEO_BLACK_SPADE_STRONG
[State -1]
type = ChangeState
value = 1108
triggerall = command = "QCB_z" && numprojid(1100) = 0
trigger1 = statetype = A && ctrl = 1

;NEO_BLACK_SPADE_MEDIUM
[State -1]
type = ChangeState
value = 1109
triggerall = command = "QCB_y" && numprojid(1100) = 0
trigger1 = statetype = A && ctrl = 1

;NEO_BLACK_SPADE_LIGHT
[State -1]
type = ChangeState
value = 1110
triggerall = command = "QCB_x" && numprojid(1100) = 0
trigger1 = statetype = A && ctrl = 1

;PLASMA_TSUNAMI_MEDIUM
[State -1]
type = ChangeState
value = 1114
triggerall = command = "QCB_y" && numprojid(1110) = 0
trigger1 = statetype = S && ctrl = 1
trigger2 = stateno = 1900 && time > 7
trigger3 = stateno = 6740 && movecontact = 1

;MONK_SUMMON_LIGHT
[State -1]
type = ChangeState
value = 1125
triggerall = command = "QCB_x" && numprojid(1120) = 0
trigger1 = statetype = S && ctrl = 1
trigger2 = stateno = 1900 && time > 7
trigger3 = stateno = 6740 && movecontact = 1

;FLYING_DRAGON_KICK
[State -1]
type = ChangeState
value = 614
triggerall = command = "QCB_a"
trigger1 = statetype = S && ctrl = 1
trigger2 = stateno = 1900 && time > 7
trigger3 = stateno = 6740 && movecontact = 1

;FLYING_DRAGON_KICK_MIDAIR
[State -1]
type = ChangeState
value = 625
triggerall = command = "QCB_a"
trigger1 = statetype = A
trigger1 = ctrl = 1
trigger2 = stateno = 615 && movecontact = 1 && time > 21
trigger3 = stateno = 600 && movecontact = 1
trigger4 = stateno = 610 && movecontact = 1

;---------------------------------------------------------------------------
;TAUNT
[State -1]
type = ChangeState
value = 700
triggerall = command != "holddown" && statetype = S && ctrl = 1
trigger1 = command = "start"

;---------------------------------------------------------------------------
;STAND_UP_ATTACK
[State -1]
type = ChangeState
value = 635
triggerall = stateno = 5120
trigger1 = command = "z" && command = "holdback"
trigger2 = var(40) = 1 && P2BodyDist X < 30 && random < 500 && time = 1

;---------------------------------------------------------------------------
;juggle_starter_human
[State -1]
type = NULL;ChangeState
value = 6700
triggerall = command = "juggle" && command != "holddown" && var(40) = 0
trigger1 = statetype = S && ctrl = 1
trigger2 = stateno = 220 && movecontact = 1
trigger3 = stateno = 420 && movecontact = 1
trigger4 = stateno = 200 && movecontact = 1
trigger5 = stateno = 400 && movecontact = 1
trigger6 = stateno = 1900 && time > 7
;trigger7 = stateno = 6740 && movecontact = 1 && var(13) < 3

;juggle_starter_CPU
[State -1]
type = NULL;ChangeState
value = 6700
triggerall = var(40) = 1 && p2bodydist X < 100 && p2statetype = S && random < 20 || var(40) = 1 && stateno = 6710 && movecontact = 1 && random < 800
trigger1 = statetype = S && ctrl = 1
trigger2 = stateno = 220 && movecontact = 1
trigger3 = stateno = 420 && movecontact = 1
trigger4 = stateno = 200 && movecontact = 1
trigger5 = stateno = 400 && movecontact = 1
trigger6 = stateno = 1900 && time > 7
;trigger7 = stateno = 6740 && movecontact = 1 && var(13) < 3

;juggle_phase#2
[State -1]
type = ChangeState
value = 6710
trigger1 = command = "x" && command != "holddown" && stateno = 6700 && movecontact = 1
trigger2 = var(40) = 1 && stateno = 6700 && movecontact = 1

;juggle_phase#3_a_(uppercut)
[State -1]
type = ChangeState
value = 6720
trigger1 = command = "z" && command != "holddown" && stateno = 6710 && movecontact = 1
trigger2 = var(40) = 1 && stateno = 6710 && movecontact = 1 && random < 300

;juggle_phase#3_b_(free-kick)
[State -1]
type = ChangeState
value = 6730
trigger1 = command = "y" && command != "holddown" && stateno = 6710 && movecontact = 1
trigger2 = var(40) = 1 && stateno = 6710 && movecontact = 1 && random < 400

;juggle_phase#3_c_(power-charge)
[State -1]
type = ChangeState
value = 6740
trigger1 = command = "x" && command != "holddown" && stateno = 6710 && movecontact = 1
trigger2 = var(40) = 1 && stateno = 6710 && movecontact = 1 && random < 500

;---------------------------------------------------------------------------
;RUN_STRONG
[State -1]
type = ChangeState
value = 235
triggerall = command = "z"
trigger1 = stateno = 100
trigger1 = ctrl = 1

;RUN_MEDIUM
[State -1]
type = ChangeState
value = 225
triggerall = command = "y"
trigger1 = stateno = 100
trigger1 = ctrl = 1

;RUN_LIGHT
[State -1]
type = ChangeState
value = 205
triggerall = command = "a"
trigger1 = stateno = 100
trigger1 = ctrl = 1

;---------------------------------------------------------------------------
;STAND_STRONG
[State -1]
type = ChangeState
value = 230
triggerall = command = "z"
triggerall = command != "holddown"
trigger1 = statetype = S
trigger1 = ctrl = 1
trigger2 = stateno = 220
trigger2 = movecontact = 1
trigger3 = stateno = 420
trigger3 = movecontact = 1
trigger4 = stateno = 200
trigger4 = movecontact = 1
trigger5 = stateno = 400
trigger5 = movecontact = 1
trigger6 = stateno = 1900 && time > 7

;STAND_MEDIUM
[State -1]
type = ChangeState
value = 220
triggerall = command = "y"
triggerall = command != "holddown"
trigger1 = statetype = S
trigger1 = ctrl = 1
trigger2 = stateno = 200
trigger2 = movecontact = 1
trigger3 = stateno = 400
trigger3 = movecontact = 1
trigger4 = stateno = 1900 && time > 7

;STAND_LIGHT
[State -1]
type = ChangeState
value = 200
triggerall = command = "x"
triggerall = command != "holddown"
trigger1 = statetype = S
trigger1 = ctrl = 1
trigger2 = stateno = 200
trigger2 = time >= 7
trigger3 = stateno = 400
trigger3 = movecontact = 1
trigger4 = stateno = 1900 && time > 7

;STAND_KICK
[State -1]
type = ChangeState
value = 210
triggerall = command = "a"
triggerall = command != "holddown"
trigger1 = statetype = S
trigger1 = ctrl = 1
trigger2 = stateno = 200
trigger2 = movecontact = 1
trigger3 = stateno = 400
trigger3 = movecontact = 1
trigger4 = stateno = 1900 && time > 7

;---------------------------------------------------------------------------
;CROUCH_LIGHT
[State -1]
type = ChangeState
value = 400
triggerall = command = "x"
triggerall = command = "holddown"
trigger1 = statetype = C
trigger1 = ctrl = 1
trigger2 = stateno = 200
trigger2 = movecontact = 1
trigger3 = stateno = 1900 && time > 7

;CROUCH_MEDIUM
[State -1]
type = ChangeState
value = 420
triggerall = command = "y"
triggerall = command = "holddown"
trigger1 = statetype = C
trigger1 = ctrl = 1
trigger2 = stateno = 220
trigger2 = movecontact = 1
trigger3 = stateno = 400
trigger3 = movecontact = 1
trigger4 = stateno = 1900 && time > 7

;CROUCH_STRONG
[State -1]
type = ChangeState
value = 430
triggerall = command = "z"
triggerall = command = "holddown"
trigger1 = statetype = C
trigger1 = ctrl = 1
trigger2 = stateno = 1900 && time > 7

;CROUCH_KICK
[State -1]
type = ChangeState
value = 410
triggerall = command = "a"
triggerall = command = "holddown"
trigger1 = statetype = C
trigger1 = ctrl = 1
trigger2 = stateno = 220
trigger2 = movecontact = 1
trigger3 = stateno = 420
trigger3 = movecontact = 1
trigger4 = stateno = 200
trigger4 = movecontact = 1
trigger5 = stateno = 400
trigger5 = movecontact = 1
trigger6 = stateno = 1900 && time > 7

;---------------------------------------------------------------------------
;JUMP_LIGHT
[State -1]
type = ChangeState
value = 600
trigger1 = command = "x"
trigger1 = statetype = A
trigger1 = ctrl = 1

;JUMP_MEDIUM
[State -1]
type = ChangeState
value = 620
trigger1 = command = "y"
trigger1 = statetype = A
trigger1 = ctrl = 1

;JUMP_STRONG
[State -1]
type = ChangeState
value = 630
trigger1 = command = "z"
trigger1 = statetype = A
trigger1 = ctrl = 1

;JUMP_KICK
[State -1]
type = ChangeState
value = 610
trigger1 = command = "a"
trigger1 = statetype = A
trigger1 = ctrl = 1



