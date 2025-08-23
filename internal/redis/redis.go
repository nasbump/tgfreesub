package redis

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	redis "github.com/redis/go-redis/v9"
)

var (
	ErrRdsConnectFail = errors.New("redis connect fail")
	ErrRdsPingFail    = errors.New("redis ping fail")
	// ErrKeyNotExisted  = errors.New("rkey not existed")
	// ErrKeyReadFail    = errors.New("rkey read fail")
	// ErrKeyNoValue     = errors.New("rkey no value")
)

type RdsClient redis.Client
type ZMember redis.Z

const Nil = redis.Nil

var RdsOperateTimeout = 10 * time.Second

func InitRedis(url string) (*RdsClient, error) {
	if !strings.HasPrefix(url, "redis://") {
		url = url + "redis://"
	}
	ctx := context.Background()

	opts, err := redis.ParseURL(url)
	if err != nil {
		// log.Panicln("paruse url:", url, " failed")
		return nil, err
	}

	if rds := redis.NewClient(opts); rds == nil {
		// log.Printf("redis connect: %s, db: %d fail", url, db)
		// log.Panicln("redis connect fail")
		return nil, ErrRdsConnectFail
	} else if err := rds.Ping(ctx).Err(); err != nil {
		// log.Printf("redis ping: %s, db: %d fail", url, db)
		// log.Panicln("redis ping fail")
		return nil, ErrRdsPingFail
	} else {
		return (*RdsClient)(rds), nil
	}
}

func (r *RdsClient) ModifyKeyTtl(rKey string, ttl time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), RdsOperateTimeout)
	defer cancel()

	return r.Expire(ctx, rKey, ttl).Err()
}
func (r *RdsClient) SetAddMember(rKey string, members ...any) error {
	ctx, cancel := context.WithTimeout(context.Background(), RdsOperateTimeout)
	defer cancel()

	return r.SAdd(ctx, rKey, members...).Err()
}
func (r *RdsClient) ZsetCard(rKey string) int64 {
	ctx, cancel := context.WithTimeout(context.Background(), RdsOperateTimeout)
	defer cancel()

	return r.ZCard(ctx, rKey).Val()
}
func (r *RdsClient) ZsetRangeByScore(rKey string, rev bool, min, max, count int64) []string {
	ctx, cancel := context.WithTimeout(context.Background(), RdsOperateTimeout)
	defer cancel()

	minScore := strconv.FormatInt(min, 10)
	maxScore := "(" + strconv.FormatInt(max, 10) // 开区间
	zrngOpts := &redis.ZRangeBy{
		Min:    minScore,
		Max:    maxScore,
		Offset: 0,
		Count:  count,
	}
	if rev {
		return r.ZRevRangeByScore(ctx, rKey, zrngOpts).Val()
	}

	return r.ZRangeByScore(ctx, rKey, zrngOpts).Val()
}
func (r *RdsClient) ZsetAddMember(rKey string, score float64, member any) error {
	ctx, cancel := context.WithTimeout(context.Background(), RdsOperateTimeout)
	defer cancel()

	return r.ZAdd(ctx, rKey, redis.Z{Score: score, Member: member}).Err()
}

func (r *RdsClient) HashGetAll(rKey string, out any) error {
	ctx, cancel := context.WithTimeout(context.Background(), RdsOperateTimeout)
	defer cancel()

	return r.HGetAll(ctx, rKey).Scan(out)
}
func (r *RdsClient) HashSetAll(rKey string, in any) error {
	ctx, cancel := context.WithTimeout(context.Background(), RdsOperateTimeout)
	defer cancel()

	return r.HSet(ctx, rKey, in).Err()
}

func (r *RdsClient) CheckKeyExisted(rKey string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), RdsOperateTimeout)
	defer cancel()
	res, err := r.Exists(ctx, rKey).Result()
	return err == nil && res == 1
}
