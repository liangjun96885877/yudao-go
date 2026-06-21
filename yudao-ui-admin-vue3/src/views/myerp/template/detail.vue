<script setup lang="ts">
// myerp 产品模板(SPU)详情/新建页 —— Axelor 风格 3-tab。
//   tab1 基本信息: 共享字段(名称/分类/单位/基础售价)
//   tab2 区分属性: 选模板要用哪些 is_variant=true 的属性
//   tab3 变体清单: 列出已生成 SKU + 「按组合生成」 dialog
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, type FormInstance, type FormRules } from 'element-plus'

import { AxelorDetail } from '@/components/AxelorStyle'
import { myerpTemplateApi } from '@/api/myerp/template'
import { myerpCategoryApi } from '@/api/myerp/category'
import { myerpUomApi } from '@/api/myerp/uom'
import { myerpAttributeApi } from '@/api/myerp/attribute'
import { useRecordNav } from '@/composables/useRecordNav'
import type {
  MyErpAttribute,
  MyErpCategory,
  MyErpProduct,
  MyErpTemplate,
  MyErpTemplateAttributeLine,
  MyErpUom
} from '@/types/myerp'

defineOptions({ name: 'MyErpTemplateDetail' })

const route = useRoute()
const router = useRouter()

const isCreate = computed(() => route.path.endsWith('/create'))
const id = computed(() => Number(route.params.id))

const nav = useRecordNav('myerp:template')
const neighbors = computed(() => nav.neighbors(id.value))
const prevTo = computed(() =>
  neighbors.value.prev != null ? `/myerp/template/${neighbors.value.prev}` : null
)
const nextTo = computed(() =>
  neighbors.value.next != null ? `/myerp/template/${neighbors.value.next}` : null
)

const form = reactive<Partial<MyErpTemplate> & { attributeLines: MyErpTemplateAttributeLine[] }>({
  name: '',
  code: '',
  categoryId: 0,
  baseUomId: null,
  basePrice: '0',
  description: '',
  status: 0,
  attributeLines: []
})
const formRef = ref<FormInstance>()
const saving = ref(false)
const loading = ref(false)

const categories = ref<MyErpCategory[]>([])
const uoms = ref<MyErpUom[]>([])
// 全部 is_variant=true 的属性(候选区分属性)
const variantAttrs = ref<MyErpAttribute[]>([])
const variants = ref<MyErpProduct[]>([])

const rules: FormRules = {
  name: [{ required: true, message: '请输入模板名称', trigger: 'blur' }],
  categoryId: [
    {
      validator: (_r, v, cb) => (v > 0 ? cb() : cb(new Error('请选择分类'))),
      trigger: 'change'
    }
  ]
}

const title = computed(() =>
  isCreate.value ? '新建模板' : `模板 #${id.value} ${form.name ? `· ${form.name}` : ''}`
)

async function loadCats() {
  categories.value = await myerpCategoryApi.tree()
}
async function loadUoms() {
  uoms.value = await myerpUomApi.listAll()
}

// 加载该分类(含继承链)下的属性,筛 is_variant=true 作为候选
async function loadVariantAttrs(categoryId: number) {
  if (!categoryId) {
    variantAttrs.value = []
    return
  }
  try {
    const list = await myerpAttributeApi.listByCategory(categoryId)
    variantAttrs.value = list.filter((a) => a.isVariant && a.inputType === 'select')
  } catch {
    variantAttrs.value = []
  }
}

async function onCategoryChange(categoryId: number) {
  await loadVariantAttrs(categoryId)
  // 切分类:清掉不在新候选集里的 attributeLines
  const allowed = new Set(variantAttrs.value.map((a) => a.id))
  form.attributeLines = form.attributeLines.filter((l) => allowed.has(l.attributeId))
}

async function loadDetail() {
  if (isCreate.value) return
  loading.value = true
  try {
    const t = await myerpTemplateApi.get(id.value)
    Object.assign(form, t)
    form.attributeLines = (t.attributeLines || []).map((l) => ({ ...l }))
    await loadVariantAttrs(t.categoryId)
    variants.value = await myerpTemplateApi.listVariants(id.value)
  } catch (e: any) {
    ElMessage.error(e?.message || '加载模板失败')
  } finally {
    loading.value = false
  }
}

// 添加一行区分属性
function addAttrLine(attrId: number) {
  if (form.attributeLines.some((l) => l.attributeId === attrId)) {
    ElMessage.warning('该属性已加入')
    return
  }
  const a = variantAttrs.value.find((x) => x.id === attrId)
  if (!a) return
  form.attributeLines.push({
    attributeId: a.id,
    attributeName: a.name,
    attributeCode: a.code,
    sort: form.attributeLines.length,
    options: (a.options || []).map((o) => ({ ...o }))
  })
}
function removeAttrLine(idx: number) {
  form.attributeLines.splice(idx, 1)
}

async function onSave() {
  if (!formRef.value) return
  try {
    await formRef.value.validate()
  } catch {
    return
  }
  const payload = {
    ...form,
    attributeLines: form.attributeLines.map((l, i) => ({
      attributeId: l.attributeId,
      sort: i
    }))
  }
  saving.value = true
  try {
    if (isCreate.value) {
      const newId = await myerpTemplateApi.create(payload)
      ElMessage.success('创建成功')
      router.replace(`/myerp/template/${newId}`)
    } else {
      await myerpTemplateApi.update({ ...payload, id: id.value })
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
    await myerpTemplateApi.delete(id.value)
    ElMessage.success('已删除')
    router.replace('/myerp/template')
  } catch (e: any) {
    ElMessage.error(e?.message || '删除失败')
  }
}
function onRefresh() {
  loadDetail()
}

// === 按组合生成 SKU dialog ===
const genDialog = reactive({
  visible: false,
  // 每个 attributeId 选了哪些 option value
  selections: {} as Record<string, string[]>
})
const genSaving = ref(false)

function openGen() {
  // 默认全部选项都勾上
  const sel: Record<string, string[]> = {}
  for (const l of form.attributeLines) {
    sel[String(l.attributeId)] = (l.options || []).map((o) => o.value)
  }
  genDialog.selections = sel
  genDialog.visible = true
}

const genCombinationCount = computed(() => {
  let n = 1
  for (const l of form.attributeLines) {
    const picked = genDialog.selections[String(l.attributeId)]?.length || 0
    if (picked === 0) return 0
    n *= picked
  }
  return n
})

async function submitGen() {
  if (genCombinationCount.value === 0) {
    ElMessage.warning('每个属性至少选 1 个值')
    return
  }
  genSaving.value = true
  try {
    const res = await myerpTemplateApi.generateVariants({
      templateId: id.value,
      selections: genDialog.selections
    })
    ElMessage.success(`生成 ${res.created} 个新 SKU,跳过已存在 ${res.skipped} 个`)
    genDialog.visible = false
    await loadDetail()
  } catch (e: any) {
    ElMessage.error(e?.message || '生成失败')
  } finally {
    genSaving.value = false
  }
}

function goVariant(p: MyErpProduct) {
  router.push(`/myerp/product/${p.id}`)
}

function variantCombo(p: MyErpProduct): string {
  if (!p.attrs) return '—'
  const parts: string[] = []
  for (const l of form.attributeLines) {
    const v = p.attrs[l.attributeCode]
    if (v) parts.push(`${l.attributeName}=${v}`)
  }
  return parts.length ? parts.join(' / ') : '—'
}

onMounted(async () => {
  await Promise.all([loadCats(), loadUoms()])
  if (!isCreate.value) loadDetail()
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
    :show-chatter="false"
    :show-delete="!isCreate"
    :show-refresh="!isCreate"
    :prev-to="isCreate ? undefined : prevTo"
    :next-to="isCreate ? undefined : nextTo"
    :nav-index="isCreate ? -1 : neighbors.index"
    :nav-total="neighbors.total"
    create-route="/myerp/template/create"
    back-to="/myerp/template"
    @save="onSave"
    @delete="onDelete"
    @refresh="onRefresh"
  >
    <el-form ref="formRef" :model="form" :rules="rules" label-width="100px" v-loading="loading">
      <el-tabs>
        <!-- ① 基本信息 -->
        <el-tab-pane label="基本信息">
          <el-alert
            type="info"
            :closable="false"
            show-icon
            class="tpl-tip"
          >
            此页字段由本模板所有变体(SKU)共享。改一处全部生效。
          </el-alert>
          <el-row :gutter="16">
            <el-col :span="12">
              <el-form-item label="模板名称" prop="name">
                <el-input v-model="form.name" placeholder="如:手工皂" maxlength="128" />
              </el-form-item>
            </el-col>
            <el-col :span="12">
              <el-form-item label="编码">
                <el-input v-model="form.code" placeholder="如:SOAP" maxlength="64" />
              </el-form-item>
            </el-col>
            <el-col :span="12">
              <el-form-item label="分类" prop="categoryId">
                <el-select
                  v-model="form.categoryId"
                  placeholder="请选择"
                  filterable
                  @change="onCategoryChange"
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
              <el-form-item label="基本单位">
                <el-select v-model="form.baseUomId" placeholder="请选择" clearable filterable>
                  <el-option
                    v-for="u in uoms"
                    :key="u.id"
                    :value="u.id"
                    :label="`${u.name}${u.code ? ` (${u.code})` : ''}`"
                  />
                </el-select>
              </el-form-item>
            </el-col>
            <el-col :span="12">
              <el-form-item label="基础售价">
                <el-input v-model="form.basePrice" placeholder="0.00">
                  <template #prepend>¥</template>
                </el-input>
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
              <el-form-item label="描述">
                <el-input
                  v-model="form.description"
                  type="textarea"
                  :rows="2"
                  maxlength="1024"
                  show-word-limit
                />
              </el-form-item>
            </el-col>
          </el-row>
        </el-tab-pane>

        <!-- ② 区分属性 -->
        <el-tab-pane label="区分属性">
          <div class="tpl-attr-head">
            <span>本模板用的区分属性(决定生成哪些 SKU)</span>
            <el-select
              placeholder="+ 添加区分属性"
              size="small"
              style="width: 220px"
              :model-value="undefined"
              @update:model-value="addAttrLine"
            >
              <el-option
                v-for="a in variantAttrs"
                :key="a.id"
                :value="a.id"
                :label="`${a.name} (#${a.id})`"
              />
            </el-select>
          </div>
          <el-table :data="form.attributeLines" size="small" border>
            <el-table-column label="属性" min-width="160">
              <template #default="{ row }">{{ row.attributeName }}</template>
            </el-table-column>
            <el-table-column label="可选值(含加价)" min-width="320">
              <template #default="{ row }">
                <el-tag
                  v-for="o in row.options || []"
                  :key="o.value"
                  size="small"
                  class="tpl-opt"
                >
                  {{ o.value }}<span v-if="Number(o.priceExtra) > 0">(+¥{{ Number(o.priceExtra).toFixed(2) }})</span>
                </el-tag>
                <span v-if="!row.options?.length" class="tpl-hint">该属性无选项</span>
              </template>
            </el-table-column>
            <el-table-column label="操作" width="70" align="center">
              <template #default="{ $index }">
                <el-button link type="danger" @click="removeAttrLine($index)">删除</el-button>
              </template>
            </el-table-column>
            <template #empty>
              <span class="tpl-hint">
                暂未选区分属性。需先在「属性管理」把某属性(select 类型)勾选「区分属性」,这里才能加入。
              </span>
            </template>
          </el-table>
          <div v-if="!isCreate && form.attributeLines.length > 0" class="tpl-gen-row">
            <el-button type="primary" @click="openGen">▶ 按属性组合生成 SKU</el-button>
            <span class="tpl-hint">先保存当前配置,再按勾选的值组合一次性建变体</span>
          </div>
        </el-tab-pane>

        <!-- ③ 变体清单 -->
        <el-tab-pane v-if="!isCreate" :label="`变体清单 ${variants.length}`">
          <div class="tpl-attr-head">
            <span>本模板下的 SKU(共 {{ variants.length }} 个)</span>
          </div>
          <el-table :data="variants" size="small" border>
            <el-table-column prop="id" label="编号" width="80" />
            <el-table-column label="属性组合" min-width="240">
              <template #default="{ row }">{{ variantCombo(row) }}</template>
            </el-table-column>
            <el-table-column prop="code" label="SKU 编码" min-width="160" />
            <el-table-column label="售价" width="100" align="right">
              <template #default="{ row }">¥{{ row.salePrice }}</template>
            </el-table-column>
            <el-table-column label="库存" width="120" align="right">
              <template #default="{ row }">
                {{ row.stock }}
                <span v-if="row.baseUomName" class="tpl-hint">{{ row.baseUomName }}</span>
              </template>
            </el-table-column>
            <el-table-column label="操作" width="80" align="center">
              <template #default="{ row }">
                <el-button link type="primary" @click="goVariant(row)">编辑 →</el-button>
              </template>
            </el-table-column>
            <template #empty>
              <span class="tpl-hint">暂无变体,去「区分属性」配置后点「按属性组合生成 SKU」</span>
            </template>
          </el-table>
        </el-tab-pane>
      </el-tabs>
    </el-form>
  </AxelorDetail>

  <!-- 按组合生成 SKU dialog -->
  <el-dialog v-model="genDialog.visible" title="按属性组合生成 SKU" width="640px">
    <div class="tpl-hint" style="margin-bottom: 14px">
      勾选每个属性下要参与组合的值,系统会做笛卡尔积生成 N 个新 SKU(已存在的组合自动跳过)。
    </div>
    <el-form label-width="120px">
      <el-form-item
        v-for="l in form.attributeLines"
        :key="l.attributeId"
        :label="l.attributeName"
      >
        <el-checkbox-group v-model="genDialog.selections[String(l.attributeId)]">
          <el-checkbox
            v-for="o in l.options || []"
            :key="o.value"
            :value="o.value"
            border
          >
            {{ o.value }}<span v-if="Number(o.priceExtra) > 0"> +¥{{ Number(o.priceExtra).toFixed(2) }}</span>
          </el-checkbox>
        </el-checkbox-group>
      </el-form-item>
    </el-form>
    <div class="tpl-gen-summary">
      将生成组合数:<b>{{ genCombinationCount }}</b>
    </div>
    <template #footer>
      <el-button @click="genDialog.visible = false">取消</el-button>
      <el-button
        type="primary"
        :loading="genSaving"
        :disabled="genCombinationCount === 0"
        @click="submitGen"
      >确定生成</el-button>
    </template>
  </el-dialog>
</template>

<style scoped>
.tpl-tip {
  margin-bottom: 14px;
}
.tpl-attr-head {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin: 4px 0 10px;
  font-weight: 600;
}
.tpl-opt {
  margin-right: 6px;
  margin-bottom: 4px;
}
.tpl-hint {
  font-size: 12px;
  color: var(--el-text-color-secondary);
}
.tpl-gen-row {
  margin-top: 14px;
  display: flex;
  gap: 12px;
  align-items: center;
}
.tpl-gen-summary {
  margin-top: 4px;
  text-align: right;
  color: var(--el-text-color-secondary);
}
</style>
