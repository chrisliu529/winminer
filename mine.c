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
#endif //#if LEVEL

#define N_ELEM (N_ROW * N_COLUMN)

#define ON_BOMB -1

enum BlockState {
  UNKNOWN, //the default state is it, this arrangement is more convenient for init 
  BOMB,
  SAFE
};

struct fieldBlock {
  int elem; //shown number in the block, only valid when digged
  int digged;
  enum BlockState state; //only valid when not digged    
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
  struct Position pos[8];     //unknown blocks around center
  int num;                    //number of unknown blocks
  int weight;                 //number of unknown bombs
};

static struct fieldBlock field[N_ROW][N_COLUMN];

static int unknownBlocks;
static int unknownBombs;

//auto miner result storage
static struct Position safePos[100];
static int nSafe = 0;

//------interface
static void generateBombs(void);
static int bombsInField(void);
static void markHints(void);
static int sumNeighborBombs(int row, int column);

void restart(void);
int isAvailRound(void);

typedef int (*Condition) (int row, int col);
static int getDigged(int row, int col);
static int getUndigged(int row, int col);
static int getBomb(int row, int col);
static int getUnknown(int row, int col);

//-----analyse
static int mineAt(const struct Position *pos);
static int chainOpen(int row, int col);
static int getNearbyBlocks(int row, int col, struct Position *nearby);
static int actAutomatic(struct Position *pos);
static int detectBomb(int row, int col, int undigged,
                      struct Position *bombPos);
static int getSuperRings(const struct WeightRing *wr,
                         struct WeightRing rings[]);
static void insertSafe(const struct Position *pos);
static int fetchSafe(struct Position *pos);
static void detectRingSafe(const struct WeightRing *super,
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

typedef void (*SearchMethod) (struct Position *pos);
static void topLeft(struct Position *pos);
static void topRight(struct Position *pos);
static void bottomLeft(struct Position *pos);
static void bottomRight(struct Position *pos);
static void randomHit(struct Position *pos);
static const SearchMethod sm[] =
  { topLeft, bottomRight, topRight, bottomLeft, randomHit };

//-----bench
static int nSureHit = 0;
static int nGuessHit = 0;
int milliTime(void);

int main() {
  int ret, ret2;
  struct Position pos;
  int nb = 100, nw = 0, nf = 0, ar = 0;
  int t1, t2;

  srand(time(NULL));
  t1 = milliTime();
  for (int i = 0; i < nb; i++) {
    restart();
    ret = ret2 = 0;
    while ((unknownBlocks > unknownBombs) && (ret != ON_BOMB)) {
      ret2 = actAutomatic(&pos);
      ret = mineAt(&pos);
    }
    if (ret == ON_BOMB) {
      assert(ret2 == -1); //it must be a guessed block!
      nf++;
    } else if (unknownBlocks == unknownBombs) {
      nw++;
    }
    if (isAvailRound()) {
      ar++;
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

void restart(void) {
  memset(field, 0, sizeof(field));
  nSafe = 0;
  //no bomb in field, everything is unknown
  unknownBombs = N_BOMB;
  unknownBlocks = N_ELEM;
  generateBombs();
  assert(bombsInField() == N_BOMB);
  markHints();
}

static int bombsInField(void) {
  int n = 0;

  for (int i = 0; i < N_ROW; i++) {
    for (int j = 0; j < N_COLUMN; j++) {
      if (BLOCK_ELEM(i, j) == ON_BOMB) {
        n++;
      }
    }
  }

  return n;
}

static void setBomb(int row, int column) {
  assert(row < N_ROW && column < N_COLUMN && BLOCK_ELEM(row, column) != ON_BOMB);
  BLOCK_SET_ELEM(row, column, ON_BOMB);
}

static void generateBombs(void) {
  int pos[N_ELEM];
  //init elements pos for ramdomly insert bomb
  for (int i = 0; i < N_ELEM; i++) {
    pos[i] = i;
  }

  int bombNum = N_BOMB;
  int elemNum = N_ELEM;
  while (bombNum > 0) {
    //insert a bomb
    int choice = rand() % elemNum;
    int bombPos = pos[choice];
    setBomb(bombPos / N_COLUMN, bombPos % N_COLUMN);
    //rearrange pos to avoid repeatly insert bomb in the same pos
    for (int i = choice; i < elemNum - 1; i++) {
      pos[i] = pos[i + 1];
    }
    elemNum--;
    bombNum--;
  }
}

static void markHints(void) {
  for (int i = 0; i < N_ROW; i++) {
    for (int j = 0; j < N_COLUMN; j++) {
      if (BLOCK_ELEM(i, j) == 0) {        //fill an indirective
        int k = sumNeighborBombs(i, j);
        BLOCK_SET_ELEM(i, j, k);
      }
    }
  }
}

static int sumNeighborBombs(int row, int col) {
  struct Position nearby[8];
  int sum = 0;
  int nNearby = getNearbyBlocks(row, col, nearby);
  for (int i = 0; i < nNearby; i++) {
    if (BLOCK_ELEM(nearby[i].row, nearby[i].column) == ON_BOMB) {
      sum++;
    }
  }

  return sum;
}

static int getNearbyBlocks(int row, int col, struct Position *nearby) {
  int rows[3], cols[3];
  rows[0] = row - 1;
  rows[1] = row;
  rows[2] = row + 1;
  cols[0] = col - 1;
  cols[1] = col;
  cols[2] = col + 1;

  int nBlock = 0;
  for (int i = 0; i < 3; i++) {
    for (int j = 0; j < 3; j++) {
      int tr = rows[i];
      int tc = cols[j];
      if (row == tr && col == tc) {       //do not count self in
        continue;
      }
      if ((tr >= 0) && (tr < N_ROW) &&
          (tc >= 0) && (tc < N_COLUMN)) {
        struct Position pos;
        pos.row = tr;
        pos.column = tc;
        nearby[nBlock++] = pos;
      }
    }
  }

  assert(nBlock <= 8);
  return nBlock;
}

static int detectBomb(int row, int col, int undigged,
                      struct Position *bombPos) {
  int n = 0;
  if (undigged == BLOCK_ELEM(row, col)) {     //all blocks around are bombs
    struct Position nearby[8];
    int nNearby = getNearbyBlocks(row, col, nearby);
    for (int i = 0; i < nNearby; i++) {
      int tc = nearby[i].column;
      int tr = nearby[i].row;
      if (!BLOCK_IS_DIGGED(tr, tc)) {
        BLOCK_MARK_BOMB(tr, tc);
        bombPos[n++] = nearby[i];
      }
    }
  }

  return n;
}

static void insertSafe(const struct Position *pos) {
  for (int i = 0; i < nSafe; i++) {
    if (memcmp(&safePos[i], pos, sizeof(struct Position)) == 0) {   //already in safepos
      return;
    }
  }
  safePos[nSafe++] = *pos;
}

static int fetchSafe(struct Position *pos) {
  if (nSafe > 0) {
    *pos = safePos[--nSafe];
    nSureHit++;
    return 0;
  }
  return -1;
}

static void detectSafe(int row, int col) {
  assert(BLOCK_IS_BOMB(row, col));
  int nBomb = 0, nUndigged = 0;
  struct Position nearbyBomb[8], nearbyUndigged[8], bombBlocks[8];
  int nNearbyBomb = getNearbyBlocks(row, col, nearbyBomb);

  for (int i = 0; i < nNearbyBomb; i++) {
    int tr = nearbyBomb[i].row;
    int tc = nearbyBomb[i].column;
    if (BLOCK_IS_DIGGED(tr, tc)
        && (BLOCK_ELEM(tr, tc) > 0)
        && ((nUndigged =
             getNearbyCond(tr, tc, nearbyUndigged, getUndigged)) > 0)) {
      nBomb = getNearbyCond(tr, tc, bombBlocks, getBomb);
      if ((nBomb == BLOCK_ELEM(tr, tc))
          && (nUndigged > nBomb)) {
        //filter the bomb from the undigged to get safe
        for (int j = 0; j < nUndigged; j++) {
          int tr2 = nearbyUndigged[j].row;
          int tc2 = nearbyUndigged[j].column;
          if (!BLOCK_IS_BOMB(tr2, tc2)) {
            BLOCK_MARK_SAFE(tr2, tc2);
            insertSafe(&nearbyUndigged[j]);
          }
        }
      }
    }
  }
}

static int mineAt(const struct Position *pos) {
  int ret = 0;
  int row = pos->row, col = pos->column;

  //ignore if repeat mine at same place
  if (field[row][col].digged) {
    return 0;
  }

  BLOCK_MARK_DIGGED(row, col);

  //mine at bomb, game over
  if (BLOCK_ELEM(row, col) == ON_BOMB) {
    ret = ON_BOMB;
  }
  //mine at a clear zone, chainly open
  if (BLOCK_ELEM(row, col) == 0) {
    chainOpen(row, col);
  }

  return ret;
}

static int chainOpen(int row, int col) {
  struct Position pos[8];
  int n = getNearbyCond(row, col, pos, getUndigged);
  for (int i = 0; i < n; i++) {
    int tr = pos[i].row;
    int tc = pos[i].column;
    if (!BLOCK_IS_DIGGED(tr, tc)) {
      BLOCK_MARK_DIGGED(tr, tc);
    }
    if (BLOCK_IS_DIGGED(tr, tc) && BLOCK_ELEM(tr, tc) == 0) {
      chainOpen(tr, tc);
    }
  }

  return 0;
}

static int actAutomatic(struct Position *pos) {
  int undigged = 0;
  int foundBomb;
  struct Position bombPos[8];
  static int nGuess = 0;
  int rescan;

  //check if there're pre-calculated safe position
  if (fetchSafe(pos) == 0) {
    return 0;
  }

  do {
    rescan = 0;
    //find seed first, seed->bomb->safe
    for (int i = 0; i < N_ROW; i++) {
      for (int j = 0; j < N_COLUMN; j++) {
        if (BLOCK_IS_DIGGED(i, j) &&
            (BLOCK_ELEM(i, j) > 0) &&
            (undigged = getNearbyCond(i, j, NULL, getUndigged)) > 0) {
          if ((foundBomb = detectBomb(i, j, undigged, bombPos)) > 0) {
            assert(foundBomb <= undigged);
            for (int k = 0; k < foundBomb; k++) {
              detectSafe(bombPos[k].row, bombPos[k].column);
              if (fetchSafe(pos) == 0) {
                return 0;
              }
            }
          }
        }
      }
    }

    //weight rings->superset rings->bomb/safe
    for (int i = 0; i < N_ROW; i++) {
      for (int j = 0; j < N_COLUMN; j++) {
        struct WeightRing wr;
        if (BLOCK_IS_DIGGED(i, j) &&
            (BLOCK_ELEM(i, j) > 0) &&
            (makeRing(i, j, &wr) > 0)) {
          struct WeightRing rings[8];
          int nRings = getSuperRings(&wr, rings);
          for (int k = 0; k < nRings; k++) {
            if (rings[k].weight == wr.weight) {     //the other blocks are clear
              detectRingSafe(&rings[k], &wr);
              if (fetchSafe(pos) == 0) {
                return 0;
              }
            }
            else if (rings[k].weight > wr.weight) {
              int subw = rings[k].weight - wr.weight;
              int subn = rings[k].num - wr.num;
              if (subn == subw) { //the other blocks are bombs
                foundBomb = detectRingBomb(&rings[k], &wr);
                assert(foundBomb == subw);
                rescan = 1;
              }
            }
          }
        }
      }
    }
  } while (rescan);

  //no absolutely safe block found, try guessing a block
  //using four corner approaching
  sm[(nGuess++) % 4](pos);
  nGuessHit++;
  return -1;
}

static void topLeft(struct Position *pos) {
  for (int i = 0; i < N_ROW; i++) {
    for (int j = 0; j < N_COLUMN; j++) {
      if ((!BLOCK_IS_BOMB(i, j)) && (!BLOCK_IS_DIGGED(i, j))) {
        pos->row = i;
        pos->column = j;
      }
    }
  }
}

static void topRight(struct Position *pos) {
  for (int i = 0; i < N_ROW; i++) {
    for (int j = N_COLUMN - 1; j > 0; j--) {
      if ((!BLOCK_IS_BOMB(i, j)) && (!BLOCK_IS_DIGGED(i, j))) {
        pos->row = i;
        pos->column = j;
      }
    }
  }
}

static void bottomLeft(struct Position *pos) {
  for (int i = N_ROW - 1; i > 0; i--) {
    for (int j = 0; j < N_COLUMN; j++) {
      if ((!BLOCK_IS_BOMB(i, j)) && (!BLOCK_IS_DIGGED(i, j))) {
        pos->row = i;
        pos->column = j;
      }
    }
  }
}

static void bottomRight(struct Position *pos) {
  for (int i = N_ROW - 1; i > 0; i--) {
    for (int j = N_COLUMN - 1; j > 0; j--) {
      if ((!BLOCK_IS_BOMB(i, j)) && (!BLOCK_IS_DIGGED(i, j))) {
        pos->row = i;
        pos->column = j;
      }
    }
  }
}

static void randomHit(struct Position *pos) {
  int ti = -1, tj = -1;
  int n = 0;

  for (int i = 0; i < N_ROW; i++) {
    for (int j = 0; j < N_COLUMN; j++) {
      if ((!BLOCK_IS_BOMB(i, j)) && (!BLOCK_IS_DIGGED(i, j))) {
        if (rand() % (++n) == 0) {
          tj = j;
          ti = i;
        }
      }
    }
  }

  assert(ti >= 0 && tj >= 0);
  pos->row = ti;
  pos->column = tj;
}

static int getNearbyCond(int row, int col, struct Position *pos,
                         Condition cond) {
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

static int getDigged(int row, int col) {
  return BLOCK_IS_DIGGED(row, col);
}

static int getUndigged(int row, int col) {
  return !BLOCK_IS_DIGGED(row, col);
}

static int getBomb(int row, int col) {
  return BLOCK_IS_BOMB(row, col);
}

static int getUnknown(int row, int col) {
  return ((!BLOCK_IS_DIGGED(row, col)) && BLOCK_IS_UNKNOWN(row, col));
}

static int getSuperRings(const struct WeightRing *wr, struct WeightRing rings[]) {
  if (wr->weight > 5) {       //not possible to have a super ring
    return 0;
  }

  struct Position centers[8];
  int m = getNearbyCond(wr->pos[0].row, wr->pos[0].column, centers, getDigged);
  if (m <= 1) {               //only the wr->center counted in
    return 0;
  }

  int nr = 0;
  for (int i = 0; i < m; i++) {
    int r = centers[i].row;
    int c = centers[i].column;
    if ((r == wr->center.row) && (c == wr->center.column)) {
      continue;
    }
    if (centerMadeRing(&centers[i], wr->pos, wr->num)) {
      struct WeightRing twr;
      makeRing(r, c, &twr);
      rings[nr++] = twr;
    }
  }
  return nr;
}

static int centerMadeRing(const struct Position *center,
                          const struct Position round[], int n) {
  for (int i = 0; i < n; i++) {
    if ((abs(center->row - round[i].row) > 1) ||
        (abs(center->column - round[i].column) > 1)) {
      return 0;
    }
  }
  return 1;
}

static int makeRing(int i, int j, struct WeightRing *wr) {
  int unknown = getNearbyCond(i, j, wr->pos, getUnknown);
  if (unknown == 0) {
    return 0;
  }
  wr->num = unknown;
  int nBomb = getNearbyCond(i, j, NULL, getBomb);
  wr->weight = BLOCK_ELEM(i, j) - nBomb;

  struct Position pos;
  pos.row = i;
  pos.column = j;
  wr->center = pos;
  return unknown;
}

static void detectRingSafe(const struct WeightRing *super,
                           const struct WeightRing *sub) {
  struct WeightRing result;
  int n = substractRing(super, sub, &result);
  for (int i = 0; i < n; i++) {
    BLOCK_MARK_SAFE(result.pos[i].row, result.pos[i].column);
    insertSafe(&result.pos[i]);
  }
}

static int detectRingBomb(const struct WeightRing *super,
                          const struct WeightRing *sub) {
  struct WeightRing result;
  int n = substractRing(super, sub, &result);
  for (int i = 0; i < n; i++) {
    BLOCK_MARK_BOMB(result.pos[i].row, result.pos[i].column);
  }
  return n;
}

static int inRing(const struct WeightRing *wr, const struct Position *pos) {
  for (int i = 0; i < wr->num; i++) {
    if ((wr->pos[i].row == pos->row) &&
        (wr->pos[i].column == pos->column)) {
      return 1;
    }
  }
  return 0;
}

static int substractRing(const struct WeightRing *super,
                         const struct WeightRing *sub,
                         struct WeightRing *result) {
  int n = 0;
  for (int i = 0; i < super->num; i++) {
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
int isAvailRound(void) {
  return (N_ELEM - unknownBlocks >= N_ELEM * AVAIL_RATIO);
}

int milliTime(void) {
  struct timeval tv;
  static int baseTime;
  static int bOnce = 1;

  if (bOnce) {
    baseTime = time(NULL);
    bOnce = 0;
  }
  gettimeofday(&tv, NULL);
  return (tv.tv_sec - baseTime)*1000 + tv.tv_usec/1000;
}
