<script setup lang="ts">
// myerp 属性详情/新建页 —— Axelor 风格 3-tab 布局。
//   tab1 基本信息:name / code / categoryId / inputType / unit / sort / status / description
//   tab2 校验规则:required / searchable / showInList / minValue/maxValue / minLength/maxLength / regex / defaultValue
//   tab3 枚举选项:仅 select/multi_select 显示,el-input-tag 风格增删
//
// 注意:不允许修改 input_type(后端拒,前端 disabled)
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, type FormInstance, type FormRules } from 'element-plus'
import { CircleClose } from '@element-plus/icons-vue'

import { AxelorDetail } from '@/components/AxelorStyle'
import Chatter from '@/components/chatter/Chatter.vue'
import { myerpAttributeApi } from '@/api/myerp/attribute'
import { myerpCategoryApi } from '@/api/myerp/category'
import { useRecordNav } from '@/composables/useRecordNav'
import type { MyErpAttribute, MyErpCategory, MyErpInputType } from '@/types/myerp'

defineOptions({ name: 'MyErpAttributeDetail' })

const route = useRoute()
const router = useRouter()

const isCreate = computed(() => route.path.endsWith('/create'))
const id = computed(() => Number(route.params.id))

const nav = useRecordNav('myerp:attribute')
const neighbors = computed(() => nav.neighbors(id.value))
const prevTo = computed(() =>
  neighbors.value.prev != null ? `/myerp/attribute/${neighbors.value.prev}` : null
)
const nextTo = computed(() =>
  neighbors.value.next != null ? `/myerp/attribute/${neighbors.value.next}` : null
)

const INPUT_TYPE_OPTIONS: Array<{ label: string; value: MyErpInputType }> = [
  { label: '文本 text', value: 'text' },
  { label: '数字 number', value: 'number' },
  { label: '单选 select', value: 'select' },
  { label: '多选 multi_select', value: 'multi_select' },
  { label: '布尔 bool', value: 'bool' },
  { label: '日期 date', value: 'date' },
  { label: '日期时间 datetime', value: 'datetime' },
  { label: '链接 url', value: 'url' },
  { label: '颜色 color', value: 'color' }
]

const form = reactive<Partial<MyErpAttribute>>({
  categoryId: 0,
  code: '',
  name: '',
  inputType: 'text',
  unit: '',
  required: false,
  searchable: false,
  showInList: true,
  isVariant: false,
  minValue: null,
  maxValue: null,
  minLength: null,
  maxLength: 1024,
  regex: '',
  defaultValue: '',
  sort: 0,
  status: 0,
  description: '',
  options: []
})
const formRef = ref<FormInstance>()
const saving = ref(false)
const loading = ref(false)
const categories = ref<MyErpCategory[]>([])

// 当前类型是否需要枚举选项
const needsOptions = computed(
  () => form.inputType === 'select' || form.inputType === 'multi_select'
)
// 当前类型是否需要数字校验字段
const isNumber = computed(() => form.inputType === 'number')
// 当前类型是否需要文本校验字段
const isText = computed(
  () => form.inputType === 'text' || form.inputType === 'url' || form.inputType === 'color'
)

const rules: FormRules = {
  name: [{ required: true, message: '请输入属性名称', trigger: 'blur' }],
  code: [
    { required: true, message: '请输入属性编码', trigger: 'blur' },
    {
      pattern: /^[a-zA-Z][a-zA-Z0-9_]{0,31}$/,
      message: '字母开头,只允许字母、数字、下划线,最长 32',
      trigger: 'blur'
    }
  ],
  categoryId: [
    {
      validator: (_r, v, cb) => (v > 0 ? cb() : cb(new Error('请选择所属分类'))),
      trigger: 'change'
    }
  ],
  inputType: [{ required: true, message: '请选择输入类型', trigger: 'change' }]
}

const title = computed(() =>
  isCreate.value
    ? '新建属性'
    : `属性 #${id.value} ${form.name ? `· ${form.name}` : ''}`
)

async function loadCategories() {
  try {
    categories.value = await myerpCategoryApi.tree()
  } catch {
    /* ignore */
  }
}

async function loadDetail() {
  if (isCreate.value) return
  loading.value = true
  try {
    const a = await myerpAttributeApi.get(id.value)
    Object.assign(form, a)
    if (!Array.isArray(form.options)) form.options = []
  } catch (e: any) {
    ElMessage.error(e?.message || '加载属性失败')
  } finally {
    loading.value = false
  }
}

// 枚举选项操作(value + priceExtra)
const newOption = ref('')
const newPriceExtra = ref('0')
function addOption() {
  const v = newOption.value.trim()
  if (!v) return
  if (!form.options) form.options = []
  if (form.options.some((o) => o.value === v)) {
    ElMessage.warning('该选项已存在')
    return
  }
  form.options.push({ value: v, priceExtra: newPriceExtra.value || '0' })
  newOption.value = ''
  newPriceExtra.value = '0'
}
function removeOption(idx: number) {
  form.options?.splice(idx, 1)
}

async function onSave() {
  if (!formRef.value) return
  try {
    await formRef.value.validate()
  } catch {
    return
  }
  if (needsOptions.value && (!form.options || form.options.length === 0)) {
    ElMessage.warning('select/multi_select 类型至少需要 1 个选项')
    return
  }
  saving.value = true
  try {
    if (isCreate.value) {
      const newId = await myerpAttributeApi.create(form)
      ElMessage.success('创建成功')
      router.replace(`/myerp/attribute/${newId}`)
    } else {
      await myerpAttributeApi.update({ ...form, id: id.value })
      ElMessage.success('已保存')
    }
  } catch (e: any) {
    ElMessage.error(e?.message || '保存失败')
  } finally {
    saving.value = false
  }
}

async function onDelete() {
  try {
    await myerpAttributeApi.delete(id.value)
    ElMessage.success('已删除')
    router.replace('/myerp/attribute')
  } catch (e: any) {
    ElMessage.error(e?.message || '删除失败')
  }
}

function onCopy() {
  const draft = { ...form, code: '' } // code 同分类唯一,清空待重填
  sessionStorage.setItem('myerp:attribute:copy', JSON.stringify(draft))
  router.push('/myerp/attribute/create')
}
function onRefresh() {
  loadDetail()
}

onMounted(() => {
  loadCategories()
  if (isCreate.value) {
    const raw = sessionStorage.getItem('myerp:attribute:copy')
    if (raw) {
      try {
        Object.assign(form, JSON.parse(raw))
      } catch {
        /* ignore */
      }
      sessionStorage.removeItem('myerp:attribute:copy')
    }
  } else {
    loadDetail()
  }
})
watch(
  () => route.params.id,
  () => !isCreate.value && loadDetail()
)
</script>

<template>
  <AxelorDetail
    :title="title"
    :loading="saving"
    :show-chatter="!isCreate"
    :show-delete="!isCreate"
    :show-copy="!isCreate"
    :show-refresh="!isCreate"
    :prev-to="isCreate ? undefined : prevTo"
    :next-to="isCreate ? undefined : nextTo"
    :nav-index="isCreate ? -1 : neighbors.index"
    :nav-total="neighbors.total"
    create-route="/myerp/attribute/create"
    back-to="/myerp/attribute"
    @save="onSave"
    @delete="onDelete"
    @copy="onCopy"
    @refresh="onRefresh"
  >
    <el-form
      ref="formRef"
      :model="form"
      :rules="rules"
      label-width="110px"
      v-loading="loading"
    >
      <el-tabs>
        <!-- ① 基本信息 -->
        <el-tab-pane label="基本信息">
          <el-row :gutter="16">
            <el-col :span="12">
              <el-form-item label="名称" prop="name">
                <el-input v-model="form.name" placeholder="如:品牌" maxlength="64" />
              </el-form-item>
            </el-col>
            <el-col :span="12">
              <el-form-item label="编码" prop="code">
                <el-input
                  v-model="form.code"
                  placeholder="如:brand"
                  maxlength="32"
                  :disabled="!isCreate"
                />
              </el-form-item>
            </el-col>
            <el-col :span="12">
              <el-form-item label="所属分类" prop="categoryId">
                <el-select
                  v-model="form.categoryId"
                  placeholder="请选择"
                  filterable
                  :disabled="!isCreate"
                >
                  <el-option
                    v-for="c in categories"
                    :key="c.id"
                    :value="c.id"
                    :label="`${c.name} (#${c.id})`"
                  />
                </el-select>
              </el-form-item>
            </el-col>
            <el-col :span="12">
              <el-form-item label="输入类型" prop="inputType">
                <el-select
                  v-model="form.inputType"
                  placeholder="请选择"
                  :disabled="!isCreate"
                >
                  <el-option
                    v-for="t in INPUT_TYPE_OPTIONS"
                    :key="t.value"
                    :value="t.value"
                    :label="t.label"
                  />
                </el-select>
                <div v-if="!isCreate" class="ax-hint">类型一经创建不允许修改</div>
              </el-form-item>
            </el-col>
            <el-col :span="12">
              <el-form-item label="单位">
                <el-input
                  v-model="form.unit"
                  placeholder="如:mm / kg / 英寸"
                  maxlength="16"
                />
              </el-form-item>
            </el-col>
            <el-col :span="12">
              <el-form-item label="排序">
                <el-input-number v-model="form.sort" :min="0" :max="9999" />
              </el-form-item>
            </el-col>
            <el-col :span="12">
              <el-form-item label="状态">
                <el-radio-group v-model="form.status">
                  <el-radio :value="0">启用</el-radio>
                  <el-radio :value="1">停用</el-radio>
                </el-radio-group>
              </el-form-item>
            </el-col>
            <el-col :span="24">
              <el-form-item label="说明">
                <el-input
                  v-model="form.description"
                  type="textarea"
                  :rows="2"
                  maxlength="255"
                  show-word-limit
                />
              </el-form-item>
            </el-col>
          </el-row>
        </el-tab-pane>

        <!-- ② 校验规则 -->
        <el-tab-pane label="校验规则">
          <el-row :gutter="16">
            <el-col :span="8">
              <el-form-item label="必填">
                <el-switch v-model="form.required" />
              </el-form-item>
            </el-col>
            <el-col :span="8">
              <el-form-item label="可搜索">
                <el-switch v-model="form.searchable" />
                <div class="ax-hint">控制列表筛选行是否可按该属性筛</div>
              </el-form-item>
            </el-col>
            <el-col :span="8">
              <el-form-item label="列表显示">
                <el-switch v-model="form.showInList" />
                <div class="ax-hint">控制产品列表是否显示该属性列</div>
              </el-form-item>
            </el-col>
            <el-col :span="24">
              <el-form-item label="区分属性">
                <el-switch v-model="form.isVariant" :disabled="form.inputType !== 'select'" />
                <div class="ax-hint">
                  开启:此属性可在模板里被选为「区分属性」,驱动 SKU 变体生成(如颜色/香型/规格)。
                  仅 select 类型支持。
                </div>
              </el-form-item>
            </el-col>

            <!-- number 类型 -->
            <el-col v-if="isNumber" :span="12">
              <el-form-item label="最小值">
                <el-input v-model="form.minValue" placeholder="如:0" clearable />
              </el-form-item>
            </el-col>
            <el-col v-if="isNumber" :span="12">
              <el-form-item label="最大值">
                <el-input v-model="form.maxValue" placeholder="如:9999" clearable />
              </el-form-item>
            </el-col>

            <!-- text 类型 -->
            <el-col v-if="isText" :span="12">
              <el-form-item label="最小长度">
                <el-input-number v-model="form.minLength" :min="0" :max="1024" />
              </el-form-item>
            </el-col>
            <el-col v-if="isText" :span="12">
              <el-form-item label="最大长度">
                <el-input-number v-model="form.maxLength" :min="1" :max="1024" />
                <div class="ax-hint">硬上限 1024(防 DoS)</div>
              </el-form-item>
            </el-col>
            <el-col v-if="isText" :span="24">
              <el-form-item label="正则">
                <el-input v-model="form.regex" placeholder="如:^[A-Z0-9]+$" />
              </el-form-item>
            </el-col>

            <el-col :span="24">
              <el-form-item label="默认值">
                <el-input v-model="form.defaultValue" maxlength="255" />
              </el-form-item>
            </el-col>
          </el-row>
        </el-tab-pane>

        <!-- ③ 枚举选项:仅 select/multi_select 显示。区分属性可设加价 -->
        <el-tab-pane v-if="needsOptions" label="枚举选项">
          <el-table :data="form.options || []" size="small" border>
            <el-table-column label="值" min-width="200">
              <template #default="{ row }">
                <el-input v-model="row.value" size="small" />
              </template>
            </el-table-column>
            <el-table-column label="加价(price_extra)" width="180">
              <template #default="{ row }">
                <el-input v-model="row.priceExtra" size="small" placeholder="0">
                  <template #prepend>¥</template>
                </el-input>
              </template>
            </el-table-column>
            <el-table-column label="操作" width="70" align="center">
              <template #default="{ $index }">
                <el-button link type="danger" @click="removeOption($index)">删除</el-button>
              </template>
            </el-table-column>
            <template #empty>
              <span class="ax-hint">暂无选项,下方录入后点「+」添加</span>
            </template>
          </el-table>

          <div class="ax-options-add">
            <el-input
              v-model="newOption"
              placeholder="新选项值,如:玫瑰"
              size="small"
              style="width: 200px"
              @keyup.enter="addOption"
            />
            <el-input
              v-model="newPriceExtra"
              placeholder="加价 0"
              size="small"
              style="width: 140px"
            >
              <template #prepend>¥</template>
            </el-input>
            <el-button type="primary" size="small" @click="addOption">+ 添加</el-button>
          </div>
          <div class="ax-hint">
            加价(price_extra)用于变体售价计算:**SKU 售价 = 模板基础价 + Σ 所选选项加价**。
            非「区分属性」也可填,但只在模板生成 SKU 时起作用。
          </div>
        </el-tab-pane>
      </el-tabs>
    </el-form>

    <!-- 右侧 chatter -->
    <template #chatter>
      <Chatter v-if="!isCreate && id" biz-type="myerp_attribute" :biz-id="id" />
    </template>
  </AxelorDetail>
</template>

<style scoped>
.ax-hint {
  margin-top: 2px;
  font-size: 12px;
  color: var(--el-text-color-secondary);
  line-height: 1.4;
}
.ax-options {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  align-items: center;
}
.ax-options-add {
  margin-top: 10px;
  display: flex;
  gap: 8px;
  align-items: center;
}
</style>
