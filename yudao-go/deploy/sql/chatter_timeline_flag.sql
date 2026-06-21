-- 时间线条目的用户标记（已读 / 重要），参考 Axelor MailFlags。
CREATE TABLE IF NOT EXISTS `chatter_timeline_flag` (
  `timeline_id`  BIGINT   NOT NULL COMMENT '时间线条目ID',
  `user_id`      BIGINT   NOT NULL COMMENT '用户ID',
  `is_read`      TINYINT  NOT NULL DEFAULT 0 COMMENT '是否已读',
  `is_important` TINYINT  NOT NULL DEFAULT 0 COMMENT '是否标记重要',
  `update_time`  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`timeline_id`, `user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='时间线条目用户标记';
