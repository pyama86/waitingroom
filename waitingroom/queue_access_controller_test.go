package waitingroom

/*
func TestAccessController_setPermittedNo(t *testing.T) {
	tests := []struct {
		name    string
		domain  string
		want    int64
		wantTTL int64
		wantErr bool
	}{
		{
			name:    "ok",
			domain:  testRandomString(20),
			wantTTL: 600,
			want:    1000,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			redisClient := testRedisClient()
			a := &AccessController{
				QueueBase: QueueBase{
					config: &Config{
						PermitUnitNumber: 1000,
						QueueEnableSec:  600,
					},
					redisClient: redisClient,
					cache:       NewCache(redisClient, &Config{}),
				},
			}
			redisClient.Set(context.Background(), tt.domain+"_permit_no", "0", 600)
			redisClient.Expire(context.Background(), tt.domain+"_permit_no", time.Second*600)
			got, ttl, err := a.setPermittedNo(context.Background(), tt.domain)
			if (err != nil) != tt.wantErr {
				t.Errorf("AccessController.setPermittedNo() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("AccessController.setPermittedNo() = %v, want %v", got, tt.want)
			}
			if ttl != tt.wantTTL {
				t.Errorf("AccessController.setPermittedNo() TTL miss match = %v, want %v", ttl, tt.wantTTL)
			}

			v, err := redisClient.Get(context.Background(), tt.domain+"_permit_no").Result()
			if (err != nil) != tt.wantErr {
				t.Errorf("AccessController.setPermittedNo() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if v != strconv.Itoa(int(tt.want)) {
				t.Errorf("AccessController.setPermittedNo() = %v, want %v", v, tt.want)
			}
		})
	}
}

func TestAccessController_Do(t *testing.T) {
	tests := []struct {
		ctx        context.Context
		beforeHook func(string, *redis.Client)
		name       string
		domain     string
		want       int
		wantErr    bool
	}{
		{
			name:   "ok",
			domain: testRandomString(20),
			beforeHook: func(key string, redisClient *redis.Client) {
				redisClient.Del(context.Background(), enableDomainKey)
				redisClient.SAdd(context.Background(), enableDomainKey, key)
				redisClient.SetEX(context.Background(), key+"_permit_no", "1", 3*time.Second)
			},
			want: 1001,
		},
		{
			name:   "skip update because not target domain",
			domain: testRandomString(20),
			beforeHook: func(key string, redisClient *redis.Client) {
				redisClient.Del(context.Background(), enableDomainKey)
				redisClient.SAdd(context.Background(), enableDomainKey, "hoge")
			},
			want: 0,
		},
		{
			name:   "skip update because not enabled domain",
			domain: testRandomString(20),
			beforeHook: func(key string, redisClient *redis.Client) {
				redisClient.Del(context.Background(), enableDomainKey)
				redisClient.SAdd(context.Background(), enableDomainKey, key)
			},
			want: 0,
		},
		{
			name:   "skip update because not before update time",
			domain: testRandomString(20),
			beforeHook: func(key string, redisClient *redis.Client) {
				redisClient.Del(context.Background(), enableDomainKey)
				redisClient.SAdd(context.Background(), enableDomainKey, key)
				redisClient.SetEX(context.Background(), key+"_permit_no", "1", 600*time.Second)
				redisClient.SetEX(context.Background(), key+"_lock_permit_no", "1", 10*time.Second)
			},
			want: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			redisClient := testRedisClient()
			a := &AccessController{
				QueueBase: QueueBase{
					config: &Config{
						PermitUnitNumber: 1000,
						QueueEnableSec:  600,
					},
					redisClient: redisClient,
					cache:       NewCache(redisClient, &Config{}),
				},
			}

			if tt.beforeHook != nil {
				tt.beforeHook(tt.domain, redisClient)
			}
			e := echo.New()
			if err := a.Do(context.Background(), e); (err != nil) != tt.wantErr {
				t.Errorf("AccessController.Do() error = %v, wantErr %v", err, tt.wantErr)
			}

			v, err := redisClient.Get(context.Background(), tt.domain+"_permit_no").Result()
			if tt.want != 0 {
				if (err != nil) != tt.wantErr {
					t.Errorf("AccessController.setPermittedNo() error = %v, wantErr %v", err, tt.wantErr)
					return
				}
				if v != strconv.Itoa(int(tt.want)) {
					t.Errorf("AccessController.setPermittedNo() = %v, want %v", v, tt.want)
				}

			} else {
				if err != redis.Nil {
					t.Errorf("AccessController.setPermittedNo() value = %v error = %v", v, err)
					return
				}
			}
		})
	}
}
*/
