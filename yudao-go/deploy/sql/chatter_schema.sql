-- =============================================================
-- chatter 业务时间线模块 —— 数据库表结构
-- 目标库：MySQL 8.0，字符集 utf8mb4
-- 多态键：biz_type(VARCHAR) + biz_id(BIGINT)；全部表含 tenant_id
-- 逻辑删除：deleted TINYINT（0 未删 / 1 已删），与 GORM soft_delete flag 对齐
-- =============================================================

-- 1. 时间线主表（活动流）
CREATE TABLE IF NOT EXISTS `chatter_timeline` (
  `id`            BIGINT       NOT NULL AUTO_INCREMENT COMMENT '主键',
  `tenant_id`     BIGINT       NOT NULL DEFAULT 0       COMMENT '租户编号',
  `biz_type`      VARCHAR(64)  NOT NULL                 COMMENT '业务类型',
  `biz_id`        BIGINT       NOT NULL                 COMMENT '业务主键',
  `event_type`    VARCHAR(32)  NOT NULL                 COMMENT '事件类型',
  `event_subtype` VARCHAR(32)  NOT NULL DEFAULT ''      COMMENT '事件子类型',
  `summary`       VARCHAR(512) NOT NULL DEFAULT ''      COMMENT '人类可读摘要',
  `body`          TEXT                                  COMMENT '正文/富文本',
  `actor_type`    TINYINT      NOT NULL DEFAULT 1       COMMENT '操作者类型 1用户 2系统 3AI',
  `actor_id`      BIGINT       NOT NULL DEFAULT 0       COMMENT '操作者ID',
  `actor_name`    VARCHAR(64)  NOT NULL DEFAULT ''      COMMENT '操作者名称',
  `ref_type`      VARCHAR(32)  NOT NULL DEFAULT ''      COMMENT '关联类型 comment/audit_batch/approval',
  `ref_id`        BIGINT       NOT NULL DEFAULT 0       COMMENT '关联ID',
  `visibility`    TINYINT      NOT NULL DEFAULT 1       COMMENT '可见性 1公开 2内部',
  `event_id`      VARCHAR(64)  NOT NULL                 COMMENT '事件幂等ID',
  `creator`       VARCHAR(64)  NOT NULL DEFAULT ''      COMMENT '创建者',
  `create_time`   DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updater`       VARCHAR(64)  NOT NULL DEFAULT ''      COMMENT '更新者',
  `update_time`   DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  `deleted`       TINYINT      NOT NULL DEFAULT 0       COMMENT '逻辑删除',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_event_id` (`event_id`),
  KEY `idx_feed` (`tenant_id`, `biz_type`, `biz_id`, `id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='业务时间线';

-- 2. 字段变更审计
CREATE TABLE IF NOT EXISTS `chatter_audit_log` (
  `id`          BIGINT       NOT NULL AUTO_INCREMENT COMMENT '主键',
  `tenant_id`   BIGINT       NOT NULL DEFAULT 0       COMMENT '租户编号',
  `biz_type`    VARCHAR(64)  NOT NULL                 COMMENT '业务类型',
  `biz_id`      BIGINT       NOT NULL                 COMMENT '业务主键',
  `timeline_id` BIGINT       NOT NULL                 COMMENT '所属时间线ID（一次保存聚合）',
  `field_name`  VARCHAR(64)  NOT NULL                 COMMENT '字段名',
  `field_label` VARCHAR(128) NOT NULL DEFAULT ''      COMMENT '字段展示名',
  `old_value`   TEXT                                  COMMENT '原始旧值',
  `new_value`   TEXT                                  COMMENT '原始新值',
  `old_display` VARCHAR(512) NOT NULL DEFAULT ''      COMMENT '旧值展示（枚举/外键解析）',
  `new_display` VARCHAR(512) NOT NULL DEFAULT ''      COMMENT '新值展示',
  `value_type`  VARCHAR(32)  NOT NULL DEFAULT 'string' COMMENT '值类型',
  `create_time` DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  PRIMARY KEY (`id`),
  KEY `idx_timeline` (`timeline_id`),
  KEY `idx_biz` (`tenant_id`, `biz_type`, `biz_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='字段变更审计';

-- 3. 评论
CREATE TABLE IF NOT EXISTS `chatter_comment` (
  `id`               BIGINT      NOT NULL AUTO_INCREMENT COMMENT '主键',
  `tenant_id`        BIGINT      NOT NULL DEFAULT 0       COMMENT '租户编号',
  `biz_type`         VARCHAR(64) NOT NULL                 COMMENT '业务类型',
  `biz_id`           BIGINT      NOT NULL                 COMMENT '业务主键',
  `timeline_id`      BIGINT      NOT NULL DEFAULT 0       COMMENT '对应时间线ID',
  `parent_id`        BIGINT      NOT NULL DEFAULT 0       COMMENT '父评论ID，0为根',
  `root_id`          BIGINT      NOT NULL DEFAULT 0       COMMENT '顶层评论ID',
  `content`          TEXT                                 COMMENT '评论原文',
  `content_html`     TEXT                                 COMMENT '渲染后HTML',
  `author_id`        BIGINT      NOT NULL DEFAULT 0       COMMENT '作者ID',
  `author_name`      VARCHAR(64) NOT NULL DEFAULT ''      COMMENT '作者名称',
  `mention_user_ids` JSON                                 COMMENT '@的用户ID数组',
  `attachment_count` INT         NOT NULL DEFAULT 0       COMMENT '附件数',
  `version`          INT         NOT NULL DEFAULT 0       COMMENT '乐观锁版本',
  `edited_at`        DATETIME    NULL                     COMMENT '最后编辑时间',
  `creator`          VARCHAR(64) NOT NULL DEFAULT ''      COMMENT '创建者',
  `create_time`      DATETIME    NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updater`          VARCHAR(64) NOT NULL DEFAULT ''      COMMENT '更新者',
  `update_time`      DATETIME    NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  `deleted`          TINYINT     NOT NULL DEFAULT 0       COMMENT '逻辑删除',
  PRIMARY KEY (`id`),
  KEY `idx_biz` (`tenant_id`, `biz_type`, `biz_id`),
  KEY `idx_root` (`root_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='评论';

-- 4. 关注者
CREATE TABLE IF NOT EXISTS `chatter_follower` (
  `id`              BIGINT      NOT NULL AUTO_INCREMENT COMMENT '主键',
  `tenant_id`       BIGINT      NOT NULL DEFAULT 0       COMMENT '租户编号',
  `biz_type`        VARCHAR(64) NOT NULL                 COMMENT '业务类型',
  `biz_id`          BIGINT      NOT NULL                 COMMENT '业务主键',
  `user_id`         BIGINT      NOT NULL                 COMMENT '关注用户ID',
  `user_name`       VARCHAR(64) NOT NULL DEFAULT ''      COMMENT '关注用户名称',
  `reason`          TINYINT     NOT NULL DEFAULT 1       COMMENT '关注来源 1手动2创建人3被@4负责人5自动',
  `subscribe_types` JSON                                 COMMENT '订阅事件类型数组，空为全部',
  `muted`           TINYINT     NOT NULL DEFAULT 0       COMMENT '是否免打扰',
  `create_time`     DATETIME    NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_biz_user` (`tenant_id`, `biz_type`, `biz_id`, `user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='关注者';

-- 5. 附件关联（文件本体存于 infra 文件服务）
CREATE TABLE IF NOT EXISTS `chatter_attachment` (
  `id`            BIGINT       NOT NULL AUTO_INCREMENT COMMENT '主键',
  `tenant_id`     BIGINT       NOT NULL DEFAULT 0       COMMENT '租户编号',
  `biz_type`      VARCHAR(64)  NOT NULL                 COMMENT '业务类型',
  `biz_id`        BIGINT       NOT NULL                 COMMENT '业务主键',
  `timeline_id`   BIGINT       NOT NULL DEFAULT 0       COMMENT '关联时间线ID',
  `comment_id`    BIGINT       NOT NULL DEFAULT 0       COMMENT '关联评论ID',
  `file_id`       BIGINT       NOT NULL                 COMMENT 'infra 文件ID',
  `file_name`     VARCHAR(256) NOT NULL DEFAULT ''      COMMENT '文件名',
  `file_url`      VARCHAR(1024) NOT NULL DEFAULT ''     COMMENT '文件URL',
  `file_size`     BIGINT       NOT NULL DEFAULT 0       COMMENT '文件大小（字节）',
  `content_type`  VARCHAR(128) NOT NULL DEFAULT ''      COMMENT 'MIME 类型',
  `uploader_id`   BIGINT       NOT NULL DEFAULT 0       COMMENT '上传者ID',
  `uploader_name` VARCHAR(64)  NOT NULL DEFAULT ''      COMMENT '上传者名称',
  `create_time`   DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  PRIMARY KEY (`id`),
  KEY `idx_biz` (`tenant_id`, `biz_type`, `biz_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='附件关联';

-- 6. 系统通知（用户收件箱）
CREATE TABLE IF NOT EXISTS `chatter_notification` (
  `id`           BIGINT       NOT NULL AUTO_INCREMENT COMMENT '主键',
  `tenant_id`    BIGINT       NOT NULL DEFAULT 0       COMMENT '租户编号',
  `recipient_id` BIGINT       NOT NULL                 COMMENT '接收用户ID',
  `biz_type`     VARCHAR(64)  NOT NULL                 COMMENT '业务类型',
  `biz_id`       BIGINT       NOT NULL                 COMMENT '业务主键',
  `timeline_id`  BIGINT       NOT NULL DEFAULT 0       COMMENT '关联时间线ID',
  `type`         VARCHAR(32)  NOT NULL                 COMMENT '通知类型',
  `title`        VARCHAR(255) NOT NULL DEFAULT ''      COMMENT '标题',
  `content`      VARCHAR(512) NOT NULL DEFAULT ''      COMMENT '内容',
  `is_read`      TINYINT      NOT NULL DEFAULT 0       COMMENT '是否已读',
  `read_at`      DATETIME     NULL                     COMMENT '已读时间',
  `create_time`  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  PRIMARY KEY (`id`),
  KEY `idx_inbox` (`tenant_id`, `recipient_id`, `is_read`, `id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='系统通知';

-- 7. 事务发件箱（投递可靠性）
CREATE TABLE IF NOT EXISTS `chatter_event_outbox` (
  `id`             BIGINT       NOT NULL AUTO_INCREMENT COMMENT '主键',
  `tenant_id`      BIGINT       NOT NULL DEFAULT 0       COMMENT '租户编号',
  `event_id`       VARCHAR(64)  NOT NULL                 COMMENT '事件唯一ID',
  `topic`          VARCHAR(128) NOT NULL                 COMMENT '事件主题',
  `aggregate_type` VARCHAR(64)  NOT NULL DEFAULT ''      COMMENT '聚合类型',
  `aggregate_id`   BIGINT       NOT NULL DEFAULT 0       COMMENT '聚合ID',
  `payload`        JSON         NOT NULL                 COMMENT '事件载荷',
  `status`         TINYINT      NOT NULL DEFAULT 0       COMMENT '状态 0待发 1已发 2失败',
  `retry_count`    INT          NOT NULL DEFAULT 0       COMMENT '重试次数',
  `created_at`     DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `published_at`   DATETIME     NULL                     COMMENT '投递时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_event_id` (`event_id`),
  KEY `idx_status` (`status`, `id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='事务发件箱';

-- 8. 事件存储（预留 Event Sourcing，append-only）
CREATE TABLE IF NOT EXISTS `chatter_event_store` (
  `id`             BIGINT       NOT NULL AUTO_INCREMENT COMMENT '主键',
  `tenant_id`      BIGINT       NOT NULL DEFAULT 0       COMMENT '租户编号',
  `aggregate_type` VARCHAR(64)  NOT NULL                 COMMENT '聚合类型',
  `aggregate_id`   BIGINT       NOT NULL                 COMMENT '聚合ID',
  `sequence`       INT          NOT NULL                 COMMENT '聚合内序号',
  `event_type`     VARCHAR(64)  NOT NULL                 COMMENT '事件类型',
  `event_version`  INT          NOT NULL DEFAULT 1       COMMENT '事件版本',
  `payload`        JSON         NOT NULL                 COMMENT '事件载荷',
  `metadata`       JSON                                  COMMENT '元数据',
  `occurred_at`    DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '发生时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_agg_seq` (`aggregate_type`, `aggregate_id`, `sequence`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='事件存储（Event Sourcing 预留）';

-- 9. 消费幂等表
CREATE TABLE IF NOT EXISTS `chatter_consumed_event` (
  `consumer_group` VARCHAR(64) NOT NULL COMMENT '消费组',
  `event_id`       VARCHAR(64) NOT NULL COMMENT '事件ID',
  `consumed_at`    DATETIME    NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '消费时间',
  PRIMARY KEY (`consumer_group`, `event_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='消费幂等记录';
