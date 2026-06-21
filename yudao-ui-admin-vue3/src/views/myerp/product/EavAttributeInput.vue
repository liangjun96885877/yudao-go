<script setup lang="ts">
// EAV 动态属性输入控件 —— 按 attribute.inputType 渲染对应表单组件。
//
// v-model 值类型对齐后端 attrValues 契约:
//   text / url / color / number / select → string
//   multi_select                         → string[]
//   bool                                 → boolean
//   date                                 → 'YYYY-MM-DD' string
//   datetime                             → 'YYYY-MM-DD HH:mm:ss' string
//
// 用法:
//   <EavAttributeInput :attr="attr" v-model="form.attrValues[attr.code]" />
import { computed } from 'vue'
import type { MyErpAttribute } from '@/types/myerp'

const props = defineProps<{
  attr: MyErpAttribute
  modelValue: any
}>()
const emit = defineEmits<{
  (e: 'update:modelValue', v: any): void
}>()

const value = computed({
  get: () => props.modelValue,
  set: (v) => emit('update:modelValue', v)
})

// number 类型的占位提示(带 min/max/unit)
const numberPlaceholder = computed(() => {
  const a = props.attr
  const parts: string[] = []
  if (a.minValue != null && a.minValue !== '') parts.push(`≥${a.minValue}`)
  if (a.maxValue != null && a.maxValue !== '') parts.push(`≤${a.maxValue}`)
  return parts.length ? parts.join(' ') : '请输入数字'
})
</script>

<template>
  <!-- 文本 -->
  <el-input
    v-if="attr.inputType === 'text'"
    v-model="value"
    :placeholder="attr.description || '请输入'"
    :maxlength="attr.maxLength || 1024"
    clearable
  />

  <!-- 数字:用 el-input 而非 input-number,因为后端用 string 存储,避免精度/格式问题 -->
  <el-input
    v-else-if="attr.inputType === 'number'"
    v-model="value"
    :placeholder="numberPlaceholder"
    clearable
  >
    <template v-if="attr.unit" #append>{{ attr.unit }}</template>
  </el-input>

  <!-- 单选 -->
  <el-select
    v-else-if="attr.inputType === 'select'"
    v-model="value"
    placeholder="请选择"
    clearable
    filterable
  >
    <el-option v-for="o in attr.options || []" :key="o.value" :label="o.value" :value="o.value" />
  </el-select>

  <!-- 多选 -->
  <el-select
    v-else-if="attr.inputType === 'multi_select'"
    v-model="value"
    placeholder="请选择(可多选)"
    clearable
    filterable
    multiple
    collapse-tags
    collapse-tags-tooltip
  >
    <el-option v-for="o in attr.options || []" :key="o.value" :label="o.value" :value="o.value" />
  </el-select>

  <!-- 布尔 -->
  <el-switch v-else-if="attr.inputType === 'bool'" v-model="value" />

  <!-- 日期 -->
  <el-date-picker
    v-else-if="attr.inputType === 'date'"
    v-model="value"
    type="date"
    value-format="YYYY-MM-DD"
    placeholder="选择日期"
    clearable
  />

  <!-- 日期时间 -->
  <el-date-picker
    v-else-if="attr.inputType === 'datetime'"
    v-model="value"
    type="datetime"
    value-format="YYYY-MM-DD HH:mm:ss"
    placeholder="选择日期时间"
    clearable
  />

  <!-- 链接 -->
  <el-input
    v-else-if="attr.inputType === 'url'"
    v-model="value"
    placeholder="https://"
    clearable
  >
    <template #prepend>URL</template>
  </el-input>

  <!-- 颜色 -->
  <el-color-picker v-else-if="attr.inputType === 'color'" v-model="value" />

  <!-- 兜底 -->
  <el-input v-else v-model="value" placeholder="请输入" clearable />
</template>
