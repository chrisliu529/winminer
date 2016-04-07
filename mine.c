/*
  Copyright (c) 2004, 2016 Chris Liu
  All rights reserved.

  Redistribution and use in source and binary forms, with or without modification, are permitted provided that the following conditions are met:

  * Redistributions of source code must retain the above copyright notice, this list of conditions and the following disclaimer.
  * Redistributions in binary form must reproduce the above copyright notice, this list of conditions and the following disclaimer in the documentation and/or other materials provided with the distribution.
  * The names of its contributors may not be used to endorse or promote products derived from this software without specific prior written permission.

  THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS" AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT OWNER OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.
*/

#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <time.h>
#include <memory.h>
#include <assert.h>
#include <sys/time.h>

#define N_MAX_ROW 16
#define N_MAX_COLUMN 30
#define N_MAX_ELEM (N_MAX_ROW * N_MAX_COLUMN)

enum overState {
  NOT_OVER,
  BOMBED,
  WON
};

#define LEVEL 3

#if LEVEL==1
#define N_ROW 9
#define N_COLUMN 9
#define N_BOMB 10

#elif LEVEL==2
#define N_ROW 16
#define N_COLUMN 16
#define N_BOMB 40

#elif LEVEL==3
#define N_ROW 16
#define N_COLUMN 30
#define N_BOMB 99
#endif                          //#if LEVEL

#define N_ELEM (N_ROW * N_COLUMN)
#define DEFAULT_REC "default.sav"

#define ON_BOMB -1

enum BlockState {
  UNKNOWN,                    //the default state is it, this arrangement is more convenient for init 
  BOMB,
  SAFE
};

struct fieldBlock {
  int elem;                   //shown number in the block, only valid when digged
  int digged;
  enum BlockState state;      //only valid when not digged    
};
#define BLOCK_MARK_BOMB(row,col)	(field[(row)][(col)].state = BOMB)
#define BLOCK_MARK_SAFE(row,col)	(field[(row)][(col)].state = SAFE)
#define BLOCK_MARK_UNKNOWN(row,col)	(field[(row)][(col)].state = UNKNOWN)
#define BLOCK_MARK_DIGGED(row,col)	unknownBlocks--; field[(row)][(col)].digged = 1
#define BLOCK_IS_BOMB(row,col)	(field[(row)][(col)].state == BOMB)
#define BLOCK_IS_SAFE(row,col)	(field[(row)][(col)].state == SAFE)
#define BLOCK_IS_UNKNOWN(row,col)	(field[(row)][(col)].state == UNKNOWN)
#define BLOCK_IS_DIGGED(row,col)	(field[(row)][(col)].digged)
#define BLOCK_ELEM(row,col)		(field[(row)][(col)].elem)
#define BLOCK_SET_ELEM(row,col, e)		(field[(row)][(col)].elem = (e))

struct Position {
  int row;
  int column;
};

struct WeightRing {
  struct Position center;
  struct Position pos[8];     //blocks around center
  int num;                    //how many blocks valid in pos
  int weight;                 //number of bombs
};

static struct fieldBlock field[N_MAX_ROW][N_MAX_COLUMN];

static int unknownBlocks;
static int unknownBombs;
static enum overState isOver;

//auto miner result storage
static struct Position safePos[100];
static int nSafe = 0;

//------interface
static int generateBombs(void);
static int countBombs(void);
static int sumNeighborBombs(int row, int column);
static int dbgShowField2(void);
void dbgShowField3(void);

int restart(void);
int isAvailRound(void);
static int generateField(void);
static int outputField(void);

typedef int (*Condition) (int row, int col);
static int getDigged(int row, int col);
static int getUndigged(int row, int col);
static int getBomb(int row, int col);
static int getUnknown(int row, int col);

//-----analyse
static int mineAt(int row, int col);
static int chainOpen(int row, int col);
static int getNearbyBlocks(int row, int col, struct Position *nearby);
static int actAutomatic(int *prow, int *pcol);
static int detectBomb(int row, int col, int undigged,
                      struct Position *bombPos);
static int getSuperRings(const struct WeightRing *wr,
                         struct WeightRing rings[]);
static int insertSafe(const struct Position *pos);
static int detectRingSafe(const struct WeightRing *super,
                          const struct WeightRing *sub);
static int detectRingBomb(const struct WeightRing *super,
                          const struct WeightRing *sub);
static int centerMadeRing(const struct Position *center,
                          const struct Position round[], int n);
static int makeRing(int i, int j, struct WeightRing *wr);
static int substractRing(const struct WeightRing *super,
                         const struct WeightRing *sub,
                         struct WeightRing *result);
static int getNearbyCond(int row, int col, struct Position *pos,
                         Condition cond);

typedef int (*SearchMethod) (int *prow, int *pcol);
static int topLeft(int *prow, int *pcol);
static int topRight(int *prow, int *pcol);
static int bottomLeft(int *prow, int *pcol);
static int bottomRight(int *prow, int *pcol);
static int randomHit(int *prow, int *pcol);
static const SearchMethod sm[] =
  { topLeft, bottomRight, topRight, bottomLeft, randomHit };

//-----bench
static int nSureHit = 0;
static int nGuessHit = 0;
static void resetHitCounter(void);
int milliTime(void);

int main()
{
  int ret = 0;
  int row, col;
  int i, nb, nw, nf, ar, t1, t2, ret2 = 0;

  restart();
  nb = 100;

  nf = 0;
  nw = 0;
  ar = 0;
  resetHitCounter();
  t1 = milliTime();
  for (i = 0; i < nb; i++) {
    restart();
    ret = 0;
    while ((unknownBlocks > unknownBombs) && (ret != ON_BOMB)) {
      ret2 = actAutomatic(&row, &col);
      ret = mineAt(row, col);
    }
    if (isAvailRound()) {
      ar++;
    }
    if (ret == ON_BOMB) {
      if (ret2 == 0) {
        printf("internal judgement err.\r\n");
        dbgShowField3();
      }
      nf++;
    } else if (unknownBlocks == unknownBombs) {
      nw++;
    }
  }
  t2 = milliTime();
  //bench summary
  printf("\n========summary========\n"
         "times: %d\n"
         "won:   %d\n"
         "bomb:  %d\n"
         "sure hit:  %d\n"
         "guess hit:  %d\n"
         "avail round: %d\n"
         "winning ratio:  %.2f%%\n"
         "winning ratio in avail rounds:  %.2f%%\n"
         "time cost: %dms\n",
         nb, nw, nf, nSureHit, nGuessHit, ar,
         ((float) nw / nb) * 100, ((float) nw / ar) * 100,
         t2 - t1);

  return 0;
}

int restart(void)
{
  int ret;

  memset(field, 0, sizeof(field));
  isOver = NOT_OVER;
  nSafe = 0;
  ret = generateField();
  return ret;
}

static void resetHitCounter(void)
{
  nSureHit = 0;
  nGuessHit = 0;
}

static int generateField(void)
{
  //no bomb in field, everything is unknown
  unknownBombs = N_BOMB;
  unknownBlocks = N_ELEM;
  generateBombs();
  countBombs();

  return 0;
}

static int bombsInField(void)
{
  int i, j, n = 0;

  for (i = 0; i < N_ROW; i++) {
    for (j = 0; j < N_COLUMN; j++) {
      if (BLOCK_ELEM(i, j) == ON_BOMB) {
        n++;
      }
    }
  }

  return n;
}

static int setBomb(int row, int column)
{
  if (BLOCK_ELEM(row, column) == ON_BOMB) {
    return -1;
  }

  BLOCK_ELEM(row, column) = ON_BOMB;
  return 0;
}

static int inPrev(int pp[], int bp, int len)
{
  int i;

  for (i = 0; i < len; i++) {
    if (pp[i] == bp) {
      return 1;
    }
  }

  return 0;
}

static int generateBombs(void)
{
  int bombNum = N_BOMB;
  int elemNum = N_ELEM;
  int bombPos, i, ret;
  int row, column;
  int prevPos[N_BOMB];
  int pos[N_ELEM];
  int choice;
  static int bOnce = 0;

  if (!bOnce) {
    srand(time(NULL));
    bOnce = 1;
  }
  //init elements pos for ramdomly insert bomb
  for (i = 0; i < N_ELEM; i++) {
    pos[i] = i;
  }
  memset(prevPos, -1, sizeof(prevPos));
  while (bombNum > 0) {
    //insert a bomb
    choice = rand() % elemNum;
    bombPos = pos[choice];
    assert(!inPrev(prevPos, bombPos, N_BOMB - bombNum));
    prevPos[N_BOMB - bombNum] = bombPos;
    row = bombPos / N_COLUMN;
    assert(row < N_ROW);
    column = bombPos % N_COLUMN;
    ret = setBomb(row, column);
    assert(ret == 0);
    //rearrange pos to avoid repeatly insert bomb in the same pos
    for (i = choice; i < elemNum - 1; i++) {
      pos[i] = pos[i + 1];
    }
    elemNum--;
    bombNum--;
  }
  ret = bombsInField();
  assert(ret == N_BOMB);
  return 0;
}

static int countBombs(void)
{
  int i, j, k;

  for (i = 0; i < N_ROW; i++) {
    for (j = 0; j < N_COLUMN; j++) {
      if (BLOCK_ELEM(i, j) == 0) {        //fill an indirective
        k = sumNeighborBombs(i, j);
        BLOCK_SET_ELEM(i, j, k);
      }
    }
  }

  return 0;
}

static int dbgShowField2(void)
{
  int i, j;

  for (i = 0; i < N_ROW; i++) {
    for (j = 0; j < N_COLUMN; j++) {
      printf("%2d ", field[i][j].elem);
    }
    printf("\n");
  }

  return 0;
}

void dbgShowField3(void)
{
  char buf[2];
  outputField();
  printf("-----------------------------------\n");
  dbgShowField2();
  fflush(stdin);
  fgets(buf, 2, stdin);
}

static int sumNeighborBombs(int row, int col)
{
  int i, nNearby;
  struct Position nearby[8];
  int bombSum = 0;

  nNearby = getNearbyBlocks(row, col, nearby);
  for (i = 0; i < nNearby; i++) {
    if (field[nearby[i].row][nearby[i].column].elem < 0) {
      bombSum++;
    }
  }

  return bombSum;
}

static int outputField(void)
{
  int i, j;

  for (i = 0; i < N_ROW; i++) {
    for (j = 0; j < N_COLUMN; j++) {
      if (field[i][j].digged) {
        printf("%2d", field[i][j].elem);
      } else {
        printf("%2c", '?');
      }
    }
    printf("\n");
  }
  return 0;
}

static int getNearbyBlocks(int row, int col, struct Position *nearby)
{
  int i, j;
  int tmpRow, tmpCol;
  int nBlock = 0;
  int scanRow[3];
  int scanCol[3];
  struct Position pos;

  scanRow[0] = row - 1;
  scanRow[1] = row;
  scanRow[2] = row + 1;

  scanCol[0] = col - 1;
  scanCol[1] = col;
  scanCol[2] = col + 1;

  for (i = 0; i < 3; i++) {
    for (j = 0; j < 3; j++) {
      tmpRow = scanRow[i];
      tmpCol = scanCol[j];
      if (col == tmpCol && row == tmpRow) {       //do not count self in
        continue;
      }
      if ((tmpRow >= 0) && (tmpRow < N_ROW) &&
          (tmpCol >= 0) && (tmpCol < N_COLUMN)) {
        pos.row = tmpRow;
        pos.column = tmpCol;
        nearby[nBlock++] = pos;
      }
    }
  }

  assert(nBlock <= 8);
  return nBlock;
}

static int detectBomb(int row, int col, int undigged,
                      struct Position *bombPos)
{
  int i, nNearby;
  struct Position nearby[8];
  int tmpCol, tmpRow, nBomb = 0;

  assert(BLOCK_IS_DIGGED(row, col) && BLOCK_ELEM(row, col) > 0);

  if (undigged == BLOCK_ELEM(row, col)) {     //it's clear that every block around it is bomb
    nNearby = getNearbyBlocks(row, col, nearby);
    for (i = 0; i < nNearby; i++) {
      tmpCol = nearby[i].column;
      tmpRow = nearby[i].row;
      if (!BLOCK_IS_DIGGED(tmpRow, tmpCol)) {
        BLOCK_MARK_BOMB(tmpRow, tmpCol);
        bombPos[nBomb++] = nearby[i];
      }
    }
  }

  return nBomb;
}

static int insertSafe(const struct Position *pos)
{
  int i;

  for (i = 0; i < nSafe; i++) {
    if (memcmp(&safePos[i], pos, sizeof(struct Position)) == 0) {   //already in safepos
      return -1;
    }
  }

  safePos[nSafe++] = *pos;
  return 0;
}

int detectSafe(int row, int col)
{
  int i, j;
  int nNearbyBomb;
  struct Position nearbyBomb[8];
  struct Position nearbyUndigged[8];
  struct Position bombBlocks[8];
  int tmpCol, tmpRow;
  int nBomb = 0, nUndigged = 0;

  assert(BLOCK_IS_BOMB(row, col));
  nNearbyBomb = getNearbyBlocks(row, col, nearbyBomb);
  for (i = 0; i < nNearbyBomb; i++) {
    tmpCol = nearbyBomb[i].column;
    tmpRow = nearbyBomb[i].row;
    if (BLOCK_IS_DIGGED(tmpRow, tmpCol)
        && (BLOCK_ELEM(tmpRow, tmpCol) > 0)
        &&
        ((nUndigged =
          getNearbyCond(tmpRow, tmpCol, nearbyUndigged,
                        getUndigged)) > 0)) {
      nBomb = getNearbyCond(tmpRow, tmpCol, bombBlocks, getBomb);
      if ((nBomb == BLOCK_ELEM(tmpRow, tmpCol))
          && (nUndigged > nBomb)) {
        //filter the bomb from the undigged to get safe
        for (j = 0; j < nUndigged; j++) {
          if (!BLOCK_IS_BOMB
              (nearbyUndigged[j].row,
               nearbyUndigged[j].column)) {
            BLOCK_MARK_SAFE(nearbyUndigged[j].row,
                            nearbyUndigged[j].column);
            insertSafe(&nearbyUndigged[j]);
          }
        }
      }
    }
  }

  return nSafe;
}

static int mineAt(int row, int col)
{
  int ret = 0;

  //ignore if repeat mine at same place
  if (field[row][col].digged) {
    return 0;
  }

  BLOCK_MARK_DIGGED(row, col);

  //miner on bomb, game over
  if (BLOCK_ELEM(row, col) == ON_BOMB) {
    ret = ON_BOMB;
  }
  //mine at a clear zone, chainly open
  if (BLOCK_ELEM(row, col) == 0) {
    chainOpen(row, col);
  }

  return ret;
}

static int chainOpen(int row, int col)
{
  int n, i;
  struct Position pos[8];
  int tr, tc;

  n = getNearbyCond(row, col, pos, getUndigged);
  for (i = 0; i < n; i++) {
    tr = pos[i].row;
    tc = pos[i].column;
    if (!BLOCK_IS_DIGGED(tr, tc)) {
      BLOCK_MARK_DIGGED(tr, tc);
    }
    if (BLOCK_IS_DIGGED(tr, tc) && BLOCK_ELEM(tr, tc) == 0) {
      chainOpen(tr, tc);
    }
  }

  return 0;
}

static int actAutomatic(int *prow, int *pcol)
{
  int i, j, k;
  int undigged = 0;
  int foundBomb;
  struct Position bombPos[8];
  static struct Position pos;
  static int nGuess = 0;
  struct WeightRing wr;
  struct WeightRing rings[8];
  int nRings;
  int subn, subw;
  int rescan;

  //check if there're pre-calculated safe position
  while (nSafe > 0) {
    pos = safePos[--nSafe];
    *pcol = pos.column;
    *prow = pos.row;
    return 0;
  }

  do {
    rescan = 0;
    //find seed first, seed->bomb->safe
    for (i = 0; i < N_ROW; i++) {
      for (j = 0; j < N_COLUMN; j++) {
        if (BLOCK_IS_DIGGED(i, j) &&
            (BLOCK_ELEM(i, j) > 0) &&
            (undigged =
             getNearbyCond(i, j, NULL, getUndigged)) > 0) {
          if ((foundBomb =
               detectBomb(i, j, undigged, bombPos)) > 0) {
            assert(foundBomb <= undigged);
            for (k = 0; k < foundBomb; k++) {
              if ((nSafe =
                   detectSafe(bombPos[k].row,
                              bombPos[k].column)) > 0) {
                pos = safePos[--nSafe];
                *pcol = pos.column;
                *prow = pos.row;
                nSureHit++;
                return 0;
              }
            }
          }
        }
      }
    }

#if 1
    //weight rings->superset rings->bomb/safe
    for (i = 0; i < N_ROW; i++) {
      for (j = 0; j < N_COLUMN; j++) {
        if (BLOCK_IS_DIGGED(i, j) &&
            (BLOCK_ELEM(i, j) > 0) && (makeRing(i, j, &wr) > 0)) {
          nRings = getSuperRings(&wr, rings);
          for (k = 0; k < nRings; k++) {
            if (rings[k].weight == wr.weight) {     //the other blocks are clear
              if ((nSafe =
                   detectRingSafe(&rings[k], &wr)) > 0) {
                pos = safePos[--nSafe];
                *pcol = pos.column;
                *prow = pos.row;
                nSureHit++;
                return 0;
              }
            }
#if 1
            else if (rings[k].weight > wr.weight) {
              subw = rings[k].weight - wr.weight;
              subn = rings[k].num - wr.num;
              if (subn == subw) { //the other blocks are bombs
                foundBomb = detectRingBomb(&rings[k], &wr);
                assert(foundBomb == subw);
                rescan = 1;
              }
            }
#endif
          }
        }
      }
    }
#endif
  } while (rescan);

  //no absolutely safe block found, find a maybe safe block instead
  //using four corner approaching
  (void) sm[(nGuess++) % 4] (prow, pcol);
  nGuessHit++;
  return -1;
}

static int topLeft(int *prow, int *pcol)
{
  int i, j;

  for (i = 0; i < N_ROW; i++) {
    for (j = 0; j < N_COLUMN; j++) {
      if ((!BLOCK_IS_BOMB(i, j)) && (!BLOCK_IS_DIGGED(i, j))) {
        *pcol = j;
        *prow = i;
        return 0;
      }
    }
  }

  return 0;
}

static int topRight(int *prow, int *pcol)
{
  int i, j;

  for (i = 0; i < N_ROW; i++) {
    for (j = N_COLUMN - 1; j > 0; j--) {
      if ((!BLOCK_IS_BOMB(i, j)) && (!BLOCK_IS_DIGGED(i, j))) {
        *pcol = j;
        *prow = i;
        return 0;
      }
    }
  }

  return 0;
}

static int bottomLeft(int *prow, int *pcol)
{
  int i, j;

  for (i = N_ROW - 1; i > 0; i--) {
    for (j = 0; j < N_COLUMN; j++) {
      if ((!BLOCK_IS_BOMB(i, j)) && (!BLOCK_IS_DIGGED(i, j))) {
        *pcol = j;
        *prow = i;
        return 0;
      }
    }
  }

  return 0;
}

static int bottomRight(int *prow, int *pcol)
{
  int i, j;

  for (i = N_ROW - 1; i > 0; i--) {
    for (j = N_COLUMN - 1; j > 0; j--) {
      if ((!BLOCK_IS_BOMB(i, j)) && (!BLOCK_IS_DIGGED(i, j))) {
        *pcol = j;
        *prow = i;
        return 0;
      }
    }
  }

  return 0;
}

static int randomHit(int *prow, int *pcol)
{
  int i, j, ti = -1, tj = -1;
  int n = 0;

  for (i = 0; i < N_ROW; i++) {
    for (j = 0; j < N_COLUMN; j++) {
      if ((!BLOCK_IS_BOMB(i, j)) && (!BLOCK_IS_DIGGED(i, j))) {
        if (rand() % (++n) == 0) {
          tj = j;
          ti = i;
        }
      }
    }
  }

  assert(ti >= 0 && tj >= 0);
  *prow = ti;
  *pcol = tj;
  return 0;
}

static int getNearbyCond(int row, int col, struct Position *pos,
                         Condition cond)
{
  int i, nNearby;
  struct Position nearby[8];
  int cnt = 0;

  nNearby = getNearbyBlocks(row, col, nearby);
  for (i = 0; i < nNearby; i++) {
    if (cond(nearby[i].row, nearby[i].column)) {
      if (pos) {
        pos[cnt] = nearby[i];
      }
      cnt++;
    }
  }

  return cnt;
}

static int getDigged(int row, int col)
{
  return BLOCK_IS_DIGGED(row, col);
}

static int getUndigged(int row, int col)
{
  return !BLOCK_IS_DIGGED(row, col);
}

static int getBomb(int row, int col)
{
  return BLOCK_IS_BOMB(row, col);
}

static int getUnknown(int row, int col)
{
  return ((!BLOCK_IS_DIGGED(row, col)) && BLOCK_IS_UNKNOWN(row, col));
}

static int getSuperRings(const struct WeightRing *wr,
                         struct WeightRing rings[])
{
  int m, i;
  struct Position centers[8];
  struct WeightRing twr;
  int nr = 0;

  if (wr->weight > 5) {       //not possible to have a super ring
    return 0;
  }

  m = getNearbyCond(wr->pos[0].row, wr->pos[0].column, centers,
                    getDigged);
  if (m <= 1) {               //only the wr->center counted in
    return 0;
  }
  for (i = 0; i < m; i++) {
    if ((centers[i].row == wr->center.row) &&
        (centers[i].column == wr->center.column)) {
      continue;
    }
    if (centerMadeRing(&centers[i], wr->pos, wr->num)) {
      makeRing(centers[i].row, centers[i].column, &twr);
      rings[nr++] = twr;
    }
  }

  return nr;
}

static int centerMadeRing(const struct Position *center,
                          const struct Position round[], int n)
{
  int i;

  for (i = 0; i < n; i++) {
    if ((abs(center->row - round[i].row) > 1) ||
        (abs(center->column - round[i].column) > 1)) {
      return 0;
    }
  }

  return 1;
}

static int makeRing(int i, int j, struct WeightRing *wr)
{
  int unknown, nBomb;
  struct Position pos;

  unknown = getNearbyCond(i, j, wr->pos, getUnknown);
  if (unknown == 0) {
    return 0;
  }
  wr->num = unknown;
  nBomb = getNearbyCond(i, j, NULL, getBomb);
  wr->weight = BLOCK_ELEM(i, j) - nBomb;
  pos.column = j;
  pos.row = i;
  wr->center = pos;
  return wr->num;
}

static int detectRingSafe(const struct WeightRing *super,
                          const struct WeightRing *sub)
{
  struct WeightRing result;
  int n, i;

  n = substractRing(super, sub, &result);
  for (i = 0; i < n; i++) {
    BLOCK_MARK_SAFE(result.pos[i].row, result.pos[i].column);
    insertSafe(&result.pos[i]);
  }

  return nSafe;
}

static int detectRingBomb(const struct WeightRing *super,
                          const struct WeightRing *sub)
{
  struct WeightRing result;
  int n, i;

  n = substractRing(super, sub, &result);
  for (i = 0; i < n; i++) {
    BLOCK_MARK_BOMB(result.pos[i].row, result.pos[i].column);
  }

  return n;
}

static int inRing(const struct WeightRing *wr, const struct Position *pos)
{
  int i;

  for (i = 0; i < wr->num; i++) {
    if ((wr->pos[i].row == pos->row) &&
        (wr->pos[i].column == pos->column)) {
      return 1;
    }
  }

  return 0;
}

static int substractRing(const struct WeightRing *super,
                         const struct WeightRing *sub,
                         struct WeightRing *result)
{
  int i;
  int n = 0;

  for (i = 0; i < super->num; i++) {
    if (!inRing(sub, &super->pos[i])) {
      result->pos[n++] = super->pos[i];
    }
  }

  result->num = n;
  result->weight = super->weight - sub->weight;
  assert(result->weight >= 0);
  return n;
}

#define AVAIL_RATIO (0.1)
int isAvailRound(void)
{
  return (N_ELEM - unknownBlocks >= N_ELEM * AVAIL_RATIO);
}

static int baseTime;
static int bOnce = 1;
int milliTime(void)
{
  struct timeval tv;

  if (bOnce) {
    baseTime = time(NULL);
    bOnce = 0;
  }

  gettimeofday(&tv, NULL);

  return (tv.tv_sec - baseTime)*1000 + tv.tv_usec/1000;
}
