; 6-Button_Pad_CMD
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
name = "PANG"
command = start
time = 5

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
name = "c"
command = c
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
name = "fw"
command = F
time = 1

[Command]
name = "bw"
command = B
time = 1

[Command]
name = "up"
command = U
time = 1

[Command]
name = "dw"
command = D
time = 1

[Command]
name = "dp_z"
command = ~F, D, DF, z
time = 20

[Command]
name = "dp_y"
command = ~F, D, DF, y
time = 20

[Command]
name = "dp_x"
command = ~F, D, DF, x
time = 20

[Command]
name = "mdp_z"
command = ~B, D, DB, z
time = 20

[Command]
name = "mdp_y"
command = ~B, D, DB, y
time = 20

[Command]
name = "mdp_x"
command = ~B, D, DB, x
time = 20

[Command]
name = "QCB_z"
command = ~D, B, z
time = 15

[Command]
name = "QCB_y"
command = ~D, B, y
time = 15

[Command]
name = "QCB_x"
command = ~D, B, x
time = 15

[Command]
name = "QCB_a"
command = ~D, B, a
time = 15

[Command]
name = "QCF_z"
command = ~D, F, z
time = 15

[Command]
name = "QCF_y"
command = ~D, F, y
time = 15

[Command]
name = "QCF_x"
command = ~D, F, x
time = 15

[Command]
name = "Evade"
command = x+y
time = 15

[Command]
name = "juggle"
command = y+z
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
name = "holdx"
command = /$x
time = 1

[Command]
name = "holdy"
command = /$y
time = 1

[Command]
name = "holdz"
command = /$z
time = 1

[Command]
name = "start"
command = s
time = 1

[Command]
name = "down_z"
command = /$D,z
time = 1

[Command]
name = "charge#1"
command = x+y+z
time = 1

[Command]
name = "charge#2"
command = /$x+y+z
time = 1

[Command]
name = "counter"
command = y+z
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
;---------------------------------------------------------------------------

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

;---------------------------------------------------------------------------
;ADDITIONAL_HIT
[State -1]
type = ChangeState
value = 240
triggerall = command = "down_z" && p2stateno >= 5030 && p2stateno < 5150 && P2BodyDist X < 30
trigger1 = statetype = C && ctrl = 1

;---------------------------------------------------------------------------
;THROW
[State -1]
type = ChangeState
value = 900
triggerall = command = "z"
triggerall = statetype = S
triggerall = ctrl
triggerall = stateno != 100
trigger1 = command = "holdfwd"
trigger1 = p2bodydist X < 3
trigger1 = (p2statetype = S) || (p2statetype = C)
trigger1 = p2movetype != H
trigger2 = command = "holdback"
trigger2 = p2bodydist X < 5
trigger2 = (p2statetype = S) || (p2statetype = C)
trigger2 = p2movetype != H

;===========================================================================
;---------------------------------------------------------------------------
;(DM)THE_BIG_ONE
[State -1]
type = ChangeState
value = 6000
triggerall = command = "QCFQCF_z"
triggerall = var(50) = 2
trigger1 = statetype = S && ctrl = 1
trigger2 = stateno = 220 && MoveContact
trigger3 = stateno = 400 && MoveContact && time <= 16
trigger4 = stateno = 410 && MoveContact
trigger5 = stateno = 420 && MoveContact
trigger6 = stateno = 1900 && time > 7
trigger7 = stateno = 211 && MoveContact

;(DM)FLASH
[State -1]
type = ChangeState
value = 6200
triggerall = command = "QCFQCF_y"
triggerall = var(50) = 2
trigger1 = statetype = S && ctrl = 1
trigger2 = stateno = 220 && MoveContact
trigger3 = stateno = 400 && MoveContact && time <= 16
trigger4 = stateno = 410 && MoveContact
trigger5 = stateno = 420 && MoveContact
trigger6 = stateno = 1900 && time > 7
trigger7 = stateno = 211 && MoveContact

;(DM)BIG_ROLLING_ONE
[State -1]
type = ChangeState
value = 6400
triggerall = command = "QCFQCF_x" && numhelper(6410) = 0
triggerall = var(50) = 2
trigger1 = statetype = S && ctrl = 1
trigger2 = stateno = 220 && MoveContact
trigger3 = stateno = 400 && MoveContact && time <= 16
trigger4 = stateno = 410 && MoveContact
trigger5 = stateno = 420 && MoveContact
trigger6 = stateno = 1900 && time > 7
trigger7 = stateno = 211 && MoveContact

;(DM)THE_BIG_ONE_AI
[State -1]
type = ChangeState
value = 6000
triggerall = random < 300 && MoveHit && var(50) = 2 && var(40) = 1
trigger1 = stateno = 220
trigger2 = stateno = 400 && time <= 16
trigger3 = stateno = 410
trigger4 = stateno = 420
trigger5 = stateno = 211

;(DM)FLASH_AI
[State -1]
type = ChangeState
value = 6200
triggerall = random < 300 && MoveHit && var(50) = 2 && var(40) = 1
trigger1 = stateno = 220
trigger2 = stateno = 400 && time <= 16
trigger3 = stateno = 410
trigger4 = stateno = 420
trigger5 = stateno = 6740
trigger6 = stateno = 211

;(DM)BIG_ROLLING_ONE_AI
[State -1]
type = ChangeState
value = 6400
triggerall = random < 300 && MoveHit && var(50) = 2 && var(40) = 1 && numhelper(6410) = 0
trigger1 = stateno = 220
trigger2 = stateno = 400 && time <= 16
trigger3 = stateno = 410
trigger4 = stateno = 420
trigger5 = stateno = 6740
trigger6 = stateno = 211

;===========================================================================
;---------------------------------------------------------------------------
;CHARGE_HARD
[State -1]
type = ChangeState
value = 1000
triggerall = command = "dp_z"
trigger1 = statetype = S && ctrl = 1
trigger2 = stateno = 220 && MoveContact
trigger3 = stateno = 400 && MoveContact && time <= 16
trigger4 = stateno = 410 && MoveContact
trigger5 = stateno = 420 && MoveContact
trigger6 = stateno = 1401 && time >= 40
trigger7 = stateno = 1900 && time > 7
trigger8 = stateno = 6740 && MoveContact
trigger9 = stateno = 211 && MoveContact

;CHARGE_MEDIUM
[State -1]
type = ChangeState
value = 1010
triggerall = command = "dp_y"
trigger1 = statetype = S && ctrl = 1
trigger2 = stateno = 220 && MoveContact
trigger3 = stateno = 400 && MoveContact && time <= 16
trigger4 = stateno = 410 && MoveContact
trigger5 = stateno = 420 && MoveContact
trigger6 = stateno = 1401 && time >= 40
trigger7 = stateno = 1900 && time > 7
trigger8 = stateno = 6740 && MoveContact
trigger9 = stateno = 211 && MoveContact

;CHARGE_LIGHT
[State -1]
type = ChangeState
value = 1020
triggerall = command = "dp_x"
trigger1 = statetype = S && ctrl = 1
trigger2 = stateno = 220 && MoveContact
trigger3 = stateno = 400 && MoveContact && time <= 16
trigger4 = stateno = 410 && MoveContact
trigger5 = stateno = 420 && MoveContact
trigger6 = stateno = 1401 && time >= 40
trigger7 = stateno = 1900 && time > 7
trigger8 = stateno = 6740 && MoveContact
trigger9 = stateno = 211 && MoveContact

;AIR_CHARGE_HARD
[State -1]
type = ChangeState
value = 1100
triggerall = command = "dp_z"
trigger1 = statetype = A && ctrl = 1
trigger2 = stateno = 600 && MoveContact && time <= 10
trigger3 = stateno = 610 && MoveContact && time <= 8
trigger4 = stateno = 620 && MoveContact && time <= 8
trigger5 = stateno = 630 && MoveContact && time <= 8
trigger6 = stateno = 230 && MoveContact && time <= 10
trigger7 = stateno = 205 && MoveContact && time <= 5

;AIR_CHARGE_MEDIUM
[State -1]
type = ChangeState
value = 1110
triggerall = command = "dp_y"
trigger1 = statetype = A && ctrl = 1
trigger2 = stateno = 600 && MoveContact && time <= 10
trigger3 = stateno = 610 && MoveContact && time <= 8
trigger4 = stateno = 620 && MoveContact && time <= 8
trigger5 = stateno = 630 && MoveContact && time <= 8
trigger6 = stateno = 230 && MoveContact && time <= 10
trigger7 = stateno = 205 && MoveContact && time <= 5

;AIR_CHARGE_LIGHT
[State -1]
type = ChangeState
value = 1120
triggerall = command = "dp_x"
trigger1 = statetype = A && ctrl = 1
trigger2 = stateno = 600 && MoveContact && time <= 10
trigger3 = stateno = 610 && MoveContact && time <= 8
trigger4 = stateno = 620 && MoveContact && time <= 8
trigger5 = stateno = 630 && MoveContact && time <= 8
trigger6 = stateno = 230 && MoveContact && time <= 10
trigger7 = stateno = 205 && MoveContact && time <= 5

;BOMB_HARD
[State -1]
type = ChangeState
value = 1300
triggerall = command = "QCF_z" && numhelper(1300) < 1 + var(50)
trigger1 = statetype = S && ctrl = 1
trigger2 = stateno = 220 && MoveContact
trigger3 = stateno = 400 && MoveContact
trigger4 = stateno = 410 && MoveContact
trigger5 = stateno = 420 && MoveContact
trigger6 = stateno = 1401 && time >= 8
trigger7 = stateno = 1900 && time > 7
trigger8 = stateno = 6740 && MoveContact
trigger9 = stateno = 211 && MoveContact

;BOMB_MEDIUM
[State -1]
type = ChangeState
value = 1310
triggerall = command = "QCF_y" && numhelper(1300) < 1 + var(50)
trigger1 = statetype = S && ctrl = 1
trigger2 = stateno = 220 && MoveContact
trigger3 = stateno = 400 && MoveContact
trigger4 = stateno = 410 && MoveContact
trigger5 = stateno = 420 && MoveContact
trigger6 = stateno = 1401 && time >= 8
trigger7 = stateno = 1900 && time > 7
trigger8 = stateno = 6740 && MoveContact
trigger9 = stateno = 211 && MoveContact

;BOMB_LIGHT
[State -1]
type = ChangeState
value = 1320
triggerall = command = "QCF_x" && numhelper(1300) < 1 + var(50)
trigger1 = statetype = S && ctrl = 1
trigger2 = stateno = 220 && MoveContact
trigger3 = stateno = 400 && MoveContact
trigger4 = stateno = 410 && MoveContact
trigger5 = stateno = 420 && MoveContact
trigger6 = stateno = 1401 && time >= 8
trigger7 = stateno = 1900 && time > 7
trigger8 = stateno = 6740 && MoveContact
trigger9 = stateno = 211 && MoveContact

;NO_BOMB
[State -1]
type = ChangeState
value = 1330
triggerall = command = "QCF_z" || command = "QCF_y" || command = "QCF_x"
trigger1 = statetype = S && ctrl = 1
trigger2 = stateno = 220 && MoveContact
trigger3 = stateno = 400 && MoveContact
trigger4 = stateno = 410 && MoveContact
trigger5 = stateno = 420 && MoveContact
trigger6 = stateno = 1401 && time >= 8
trigger7 = stateno = 1900 && time > 7
trigger8 = stateno = 6740 && MoveContact
trigger9 = stateno = 211 && MoveContact

;TELE_HARD
[State -1]
type = ChangeState
value = 1700
triggerall = command = "mdp_z"
trigger1 = statetype = S && ctrl = 1
trigger2 = stateno = 220 && MoveContact
trigger3 = stateno = 400 && MoveContact
trigger4 = stateno = 410 && MoveContact
trigger5 = stateno = 420 && MoveContact
trigger6 = stateno = 1401 && time >= 40
trigger7 = stateno = 1900 && time > 7
trigger8 = stateno = 1721
trigger9 = stateno = 6740 && MoveContact
trigger10 = stateno = 211 && MoveContact

;TELE_MEDIUM
[State -1]
type = ChangeState
value = 1710
triggerall = command = "mdp_y"
trigger1 = statetype = S && ctrl = 1
trigger2 = stateno = 220 && MoveContact
trigger3 = stateno = 400 && MoveContact
trigger4 = stateno = 410 && MoveContact
trigger5 = stateno = 420 && MoveContact
trigger6 = stateno = 1401 && time >= 40
trigger7 = stateno = 1900 && time > 7
trigger8 = stateno = 1721
trigger9 = stateno = 6740 && MoveContact
trigger10 = stateno = 211 && MoveContact

;TELE_LIGHT
[State -1]
type = ChangeState
value = 1720
triggerall = command = "mdp_x"
trigger1 = statetype = S && ctrl = 1
trigger2 = stateno = 220 && MoveContact
trigger3 = stateno = 400 && MoveContact
trigger4 = stateno = 410 && MoveContact
trigger5 = stateno = 420 && MoveContact
trigger6 = stateno = 1401 && time >= 40
trigger7 = stateno = 1900 && time > 7
trigger8 = stateno = 1721
trigger9 = stateno = 6740 && MoveContact
trigger10 = stateno = 211 && MoveContact

;AIR_BOUNCE_HARD
[State -1]
type = ChangeState
value = 1200
triggerall = command = "QCB_z"
trigger1 = statetype = S && ctrl = 1
trigger2 = stateno = 220 && MoveContact
trigger3 = stateno = 400 && MoveContact
trigger4 = stateno = 410 && MoveContact
trigger5 = stateno = 420 && MoveContact
trigger6 = stateno = 1900 && time > 7
trigger7 = stateno = 1401 && time >= 8
trigger8 = stateno = 6740 && MoveContact
trigger9 = stateno = 211 && MoveContact

;AIR_BOUNCE_MEDIUM
[State -1]
type = ChangeState
value = 1210
triggerall = command = "QCB_y"
trigger1 = statetype = S && ctrl = 1
trigger2 = stateno = 220 && MoveContact
trigger3 = stateno = 400 && MoveContact
trigger4 = stateno = 410 && MoveContact
trigger5 = stateno = 420 && MoveContact
trigger6 = stateno = 1900 && time > 7
trigger7 = stateno = 1401 && time >= 8
trigger8 = stateno = 6740 && MoveContact
trigger9 = stateno = 211 && MoveContact

;AIR_BOUNCE_LIGHT
[State -1]
type = ChangeState
value = 1220
triggerall = command = "QCB_x"
trigger1 = statetype = S && ctrl = 1
trigger2 = stateno = 220 && MoveContact
trigger3 = stateno = 400 && MoveContact
trigger4 = stateno = 410 && MoveContact
trigger5 = stateno = 420 && MoveContact
trigger6 = stateno = 1900 && time > 7
trigger7 = stateno = 1401 && time >= 8
trigger8 = stateno = 6740 && MoveContact
trigger9 = stateno = 211 && MoveContact

;THROW_AWAY
[State -1]
type = ChangeState
value = 1400
triggerall = command = "QCB_a"
trigger1 = statetype = S && ctrl = 1
trigger2 = stateno = 220 && MoveContact
trigger3 = stateno = 400 && MoveContact
trigger4 = stateno = 410 && MoveContact
trigger5 = stateno = 420 && MoveContact
trigger6 = stateno = 1900 && time > 7
trigger7 = stateno = 6740 && MoveContact
trigger8 = stateno = 211 && MoveContact

;BERSERKER
[State -1]
type = ChangeState
value = 1500
triggerall = command = "charge#1" && var(50) != 2
trigger1 = statetype = S
trigger1 = ctrl = 1

;EVADE
[State -1]
type = ChangeState
value = 1900
triggerall = command = "Evade"
trigger1 = statetype = S && ctrl = 1
trigger2 = stateno = 220 && MoveContact
trigger3 = stateno = 400 && MoveContact
trigger4 = stateno = 410 && MoveContact
trigger5 = stateno = 420 && MoveContact
trigger6 = stateno = 211 && MoveContact

;===========================================================================
;---------------------------------------------------------------------------
;juggle_starter_human
[State -1]
type = NULL;ChangeState
value = 6700
triggerall = command = "juggle" && command != "holddown" && var(40) = 0
trigger1 = statetype = S && ctrl = 1
trigger2 = stateno = 220 && MoveContact
trigger3 = stateno = 400 && MoveContact && time <= 16
trigger4 = stateno = 410 && MoveContact
trigger5 = stateno = 420 && MoveContact
trigger6 = stateno = 1401 && time >= 40
trigger7 = stateno = 1900 && time > 7
;trigger8 = stateno = 6740 && MoveContact && var(16) < 3

;juggle_starter_CPU
[State -1]
type = NULL;ChangeState
value = 6700
triggerall = var(40) = 1 && p2bodydist X < 100 && p2statetype = S && random < 20 || var(40) = 1 && stateno = 6710 && MoveContact && random < 800
trigger1 = statetype = S && ctrl = 1
trigger2 = stateno = 220 && MoveContact
trigger3 = stateno = 400 && MoveContact && time <= 16
trigger4 = stateno = 410 && MoveContact
trigger5 = stateno = 420 && MoveContact
trigger6 = stateno = 1401 && time >= 40
trigger7 = stateno = 1900 && time > 7
;trigger8 = stateno = 6740 && MoveContact && var(16) < 3

;juggle_phase#2
[State -1]
type = ChangeState
value = 6710
trigger1 = command = "x" && command != "holddown" && stateno = 6700 && MoveContact
trigger2 = var(40) = 1 && stateno = 6700 && MoveContact

;juggle_phase#3_a_(uppercut)
[State -1]
type = ChangeState
value = 6720
trigger1 = command = "z" && command != "holddown" && stateno = 6710 && MoveContact
trigger2 = var(40) = 1 && stateno = 6710 && MoveContact && random < 300

;juggle_phase#3_b_(free-kick)
[State -1]
type = ChangeState
value = 6730
trigger1 = command = "y" && command != "holddown" && stateno = 6710 && MoveContact
trigger2 = var(40) = 1 && stateno = 6710 && MoveContact && random < 400

;juggle_phase#3_c_(power-charge)
[State -1]
type = ChangeState
value = 6740
trigger1 = command = "x" && command != "holddown" && stateno = 6710 && MoveContact
trigger2 = var(40) = 1 && stateno = 6710 && MoveContact && random < 500

;===========================================================================
;---------------------------------------------------------------------------
;STAND_STRONG
[State -1]
type = ChangeState
value = 200
triggerall = command = "z"
trigger1 = statetype = S && ctrl = 1
trigger2 = stateno = 220 && MoveHit
trigger3 = stateno = 420 && MoveHit
trigger4 = stateno = 410 && MoveHit
trigger5 = stateno = 1900 && time > 7
trigger6 = stateno = 211 && MoveContact

;STAND_MEDIUM
[State -1]
type = ChangeState
value = 210
triggerall = command = "y"
trigger1 = statetype = S && ctrl = 1
trigger2 = stateno = 220 && MoveHit
trigger3 = stateno = 420 && MoveHit
trigger4 = stateno = 410 && MoveHit
trigger5 = stateno = 1900 && time > 7

;STAND_LIGHT
[State -1]
type = ChangeState
value = 220
triggerall = command = "x"
trigger1 = statetype = S && ctrl = 1
trigger2 = stateno = 420 && MoveHit
trigger3 = stateno = 1900 && time > 7

;STAND_KICK
[State -1]
type = ChangeState
value = 230
triggerall = command = "a"
trigger1 = statetype = S && ctrl = 1
trigger2 = stateno = 220 && MoveHit
trigger3 = stateno = 420 && MoveHit
trigger4 = stateno = 1900 && time > 7

;CROUCH_STRONG
[State -1]
type = ChangeState
value = 400
triggerall = command = "z"
trigger1 = statetype = C && ctrl = 1
trigger2 = stateno = 220 && MoveHit
trigger3 = stateno = 420 && MoveHit
trigger4 = stateno = 410 && MoveHit
trigger5 = stateno = 1900 && time > 7

;CROUCH_MEDIUM
[State -1]
type = ChangeState
value = 410
triggerall = command = "y"
trigger1 = statetype = C && ctrl = 1
trigger2 = stateno = 420 && MoveHit
trigger3 = stateno = 220 && MoveHit
trigger4 = stateno = 1900 && time > 7

;CROUCH_LIGHT
[State -1]
type = ChangeState
value = 420
triggerall = command = "x"
trigger1 = statetype = C && ctrl = 1
trigger2 = stateno = 430 && MoveHit
trigger3 = stateno = 1900 && time > 7

;CROUCH_KICK
[State -1]
type = ChangeState
value = 430
triggerall = command = "a"
trigger1 = statetype = C && ctrl = 1
trigger2 = stateno = 1900 && time > 7

;AIR_STRONG
[State -1]
type = ChangeState
value = 600
triggerall = command = "z"
trigger1 = statetype = A && ctrl = 1
trigger2 = stateno = 620 && MoveHit

;AIR_MEDIUM
[State -1]
type = ChangeState
value = 610
triggerall = command = "y"
trigger1 = statetype = A && ctrl = 1
trigger2 = stateno = 620 && MoveHit

;AIR_LIGHT
[State -1]
type = ChangeState
value = 620
triggerall = command = "x"
trigger1 = statetype = A && ctrl = 1

;AIR_KICK
[State -1]
type = ChangeState
value = 630
triggerall = command = "a"
trigger1 = statetype = A && ctrl = 1













