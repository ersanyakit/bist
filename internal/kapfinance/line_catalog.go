package kapfinance

import (
	"sort"
	"strings"
	"sync"

	"hissebot/internal/util"
)

const (
	StatementBalanceSheet = "balance_sheet"
	StatementIncome       = "income_statement"
	StatementCashFlow     = "cash_flow_statement"
	StatementEquity       = "equity_statement"
	StatementNote         = "note"
)

type canonicalLineSearchKey struct {
	def  LineDefinition
	slug string
}

var (
	canonicalFinancialLineSearchKeysOnce  sync.Once
	canonicalFinancialLineSearchKeysCache []canonicalLineSearchKey
)

var canonicalFinancialLineCatalog = []LineDefinition{
	lineDef("1A", "Dönen Varlıklar", "Current assets", StatementBalanceSheet, "current_assets"),
	lineDef("1AA", "Nakit ve Nakit Benzerleri", "Cash and cash equivalents", StatementBalanceSheet, "cash_and_cash_equivalents", "Nakit Benzerleri"),
	lineDef("1AB", "Finansal Yatırımlar", "Short-term financial investments", StatementBalanceSheet, "short_term_financial_investments"),
	lineDef("1AC", "Ticari Alacaklar", "Short-term trade receivables", StatementBalanceSheet, "short_term_trade_receivables", "trade_receivables"),
	lineDef("1AD", "Finans Sektörü Faaliyetlerinden Alacaklar", "Short-term receivables from financial operations", StatementBalanceSheet, "short_term_financial_operations_receivables"),
	lineDef("1AE", "Diğer Alacaklar", "Short-term other receivables", StatementBalanceSheet, "short_term_other_receivables", "other_receivables"),
	lineDef("1AEA", "Müşteri Sözleşmelerinden Doğan Varlıklar", "Contract assets", StatementBalanceSheet, "contract_assets"),
	lineDef("1AF", "Stoklar", "Inventories", StatementBalanceSheet, "inventories"),
	lineDef("1AG", "Canlı Varlıklar", "Short-term biological assets", StatementBalanceSheet, "short_term_biological_assets"),
	lineDef("1AH", "Diğer Dönen Varlıklar", "Other current assets", StatementBalanceSheet, "other_current_assets"),
	lineDef("1AI", "Ara Toplam Dönen Varlıklar", "Current assets subtotal", StatementBalanceSheet, "current_assets_subtotal"),
	lineDef("1AJ", "Satış Amacıyla Elde Tutulan Duran Varlıklar", "Assets held for sale", StatementBalanceSheet, "assets_held_for_sale", "Satış Amaçlı Sınıflandırılan Duran Varlıklar"),
	lineDef("1AK", "Duran Varlıklar", "Non-current assets", StatementBalanceSheet, "non_current_assets"),
	lineDef("1B", "Uzun Vadeli Ticari Alacaklar", "Long-term trade receivables", StatementBalanceSheet, "long_term_trade_receivables"),
	lineDef("1BA", "Uzun Vadeli Finans Sektörü Faaliyetlerinden Alacaklar", "Long-term receivables from financial operations", StatementBalanceSheet, "long_term_financial_operations_receivables"),
	lineDef("1BB", "Uzun Vadeli Diğer Alacaklar", "Long-term other receivables", StatementBalanceSheet, "long_term_other_receivables"),
	lineDef("1BBA", "Uzun Vadeli Müşteri Sözleşmelerinden Doğan Varlıklar", "Long-term contract assets", StatementBalanceSheet, "long_term_contract_assets"),
	lineDef("1BC", "Uzun Vadeli Finansal Yatırımlar", "Long-term financial investments", StatementBalanceSheet, "long_term_financial_investments"),
	lineDef("1BD", "Özkaynak Yöntemiyle Değerlenen Yatırımlar", "Investments accounted for using equity method", StatementBalanceSheet, "equity_method_investments"),
	lineDef("1BE", "Uzun Vadeli Canlı Varlıklar", "Long-term biological assets", StatementBalanceSheet, "long_term_biological_assets"),
	lineDef("1BF", "Yatırım Amaçlı Gayrimenkuller", "Investment properties", StatementBalanceSheet, "investment_properties"),
	lineDef("1BFA", "Uzun Vadeli Stoklar", "Long-term inventories", StatementBalanceSheet, "long_term_inventories"),
	lineDef("1BFAA", "Kullanım Hakkı Varlıkları", "Right-of-use assets", StatementBalanceSheet, "right_of_use_assets"),
	lineDef("1BG", "Maddi Duran Varlıklar", "Property, plant and equipment", StatementBalanceSheet, "property_plant_equipment"),
	lineDef("1BGA", "Şerefiye", "Goodwill", StatementBalanceSheet, "goodwill"),
	lineDef("1BH", "Maddi Olmayan Duran Varlıklar", "Intangible assets", StatementBalanceSheet, "intangible_assets"),
	lineDef("1BJ", "Ertelenmiş Vergi Varlığı", "Deferred tax assets", StatementBalanceSheet, "deferred_tax_assets"),
	lineDef("1BK", "Diğer Duran Varlıklar", "Other non-current assets", StatementBalanceSheet, "other_non_current_assets"),
	lineDef("1BL", "TOPLAM VARLIKLAR", "Total assets", StatementBalanceSheet, "total_assets", "Varlıklar"),
	lineDef("1BM", "KAYNAKLAR", "Liabilities", StatementBalanceSheet, "liabilities"),
	lineDef("2A", "Kısa Vadeli Yükümlülükler", "Current liabilities", StatementBalanceSheet, "current_liabilities"),
	lineDef("2AA", "Kısa Vadeli Finansal Borçlar", "Short-term financial loans", StatementBalanceSheet, "short_term_financial_debt", "Kısa Vadeli Borçlanmalar", "Finansal Borçlar", "Uzun Vadeli Borçlanmaların Kısa Vadeli Kısımları"),
	lineDef("2AAG", "Diğer Finansal Yükümlülükler", "Other short-term financial liabilities", StatementBalanceSheet, "other_short_term_financial_liabilities"),
	lineDef("2AAGAA", "Kısa Vadeli Ticari Borçlar", "Short-term trade payables", StatementBalanceSheet, "short_term_trade_payables", "trade_payables", "Ticari Borçlar"),
	lineDef("2AAGAB", "Kısa Vadeli Diğer Borçlar", "Short-term other payables", StatementBalanceSheet, "short_term_other_payables"),
	lineDef("2AAGAC", "Müşteri Sözleşmelerinden Doğan Kısa Vadeli Yükümlülükler", "Short-term contract liabilities", StatementBalanceSheet, "short_term_contract_liabilities"),
	lineDef("2AAGB", "Kısa Vadeli Finans Sektörü Faaliyetlerinden Borçlar", "Short-term payables from financial operations", StatementBalanceSheet, "short_term_financial_operations_payables"),
	lineDef("2AAGC", "Kısa Vadeli Devlet Teşvik ve Yardımları", "Short-term government grants", StatementBalanceSheet, "short_term_government_grants"),
	lineDef("2AAGCA", "Kısa Vadeli Ertelenmiş Gelirler", "Short-term deferred income", StatementBalanceSheet, "short_term_deferred_income"),
	lineDef("2AAGD", "Dönem Karı Vergi Yükümlülüğü", "Current tax liabilities", StatementBalanceSheet, "current_tax_liabilities"),
	lineDef("2AAGE", "Kısa Vadeli Borç Karşılıkları", "Short-term provisions", StatementBalanceSheet, "short_term_provisions", "Kısa Vadeli Karşılıklar"),
	lineDef("2AAGF", "Diğer Kısa Vadeli Yükümlülükler", "Other short-term liabilities", StatementBalanceSheet, "other_current_liabilities"),
	lineDef("2AAGG", "Ara Toplam Kısa Vadeli Yükümlülükler", "Current liabilities subtotal", StatementBalanceSheet, "current_liabilities_subtotal"),
	lineDef("2AAGH", "Satış Amaçlı Elde Tutulan Duran Varlıklara İlişkin Yükümlülükler", "Liabilities related to assets held for sale", StatementBalanceSheet, "liabilities_related_to_assets_held_for_sale"),
	lineDef("2B", "Uzun Vadeli Yükümlülükler", "Non-current liabilities", StatementBalanceSheet, "non_current_liabilities"),
	lineDef("2BA", "Uzun Vadeli Finansal Borçlar", "Long-term financial loans", StatementBalanceSheet, "long_term_financial_debt", "Uzun Vadeli Borçlanmalar"),
	lineDef("2BB", "Uzun Vadeli Diğer Finansal Yükümlülükler", "Other long-term financial liabilities", StatementBalanceSheet, "other_long_term_financial_liabilities"),
	lineDef("2BBA", "Uzun Vadeli Ticari Borçlar", "Long-term trade payables", StatementBalanceSheet, "long_term_trade_payables"),
	lineDef("2BBB", "Uzun Vadeli Diğer Borçlar", "Long-term other payables", StatementBalanceSheet, "long_term_other_payables"),
	lineDef("2BBBA", "Müşteri Sözleşmelerinden Doğan Uzun Vadeli Yükümlülükler", "Long-term contract liabilities", StatementBalanceSheet, "long_term_contract_liabilities"),
	lineDef("2BC", "Uzun Vadeli Finans Sektörü Faaliyetlerinden Borçlar", "Long-term payables from financial operations", StatementBalanceSheet, "long_term_financial_operations_payables"),
	lineDef("2BD", "Uzun Vadeli Devlet Teşvik ve Yardımları", "Long-term government grants", StatementBalanceSheet, "long_term_government_grants"),
	lineDef("2BDA", "Uzun Vadeli Ertelenmiş Gelirler", "Long-term deferred income", StatementBalanceSheet, "long_term_deferred_income"),
	lineDef("2BE", "Uzun Vadeli Karşılıklar", "Long-term provisions", StatementBalanceSheet, "long_term_provisions"),
	lineDef("2BF", "Çalışanlara Sağlanan Faydalara İlişkin Karşılıklar", "Employee benefit obligations", StatementBalanceSheet, "employee_benefit_obligations", "Çalışanlara Sağlanan Faydalara İlişkin Karş."),
	lineDef("2BG", "Ertelenmiş Vergi Yükümlülüğü", "Deferred tax liabilities", StatementBalanceSheet, "deferred_tax_liabilities"),
	lineDef("2BH", "Diğer Uzun Vadeli Yükümlülükler", "Other long-term liabilities", StatementBalanceSheet, "other_non_current_liabilities"),
	lineDef("2N", "Özkaynaklar", "Shareholders equity", StatementBalanceSheet, "equity"),
	lineDef("2O", "Ana Ortaklığa Ait Özkaynaklar", "Parent shareholders equity", StatementBalanceSheet, "parent_equity"),
	lineDef("2OA", "Ödenmiş Sermaye", "Paid-in capital", StatementBalanceSheet, "paid_in_capital"),
	lineDef("2OC", "Karşılıklı İştirak Sermayesi Düzeltmesi (-)", "Treasury share capital adjustments", StatementBalanceSheet, "treasury_share_capital_adjustments"),
	lineDef("2OCA", "Hisse Senedi İhraç Primleri", "Share premiums", StatementBalanceSheet, "share_premiums", "Paylara İlişkin Primler"),
	lineDef("2OCB", "Değer Artış Fonları", "Revaluation reserves", StatementBalanceSheet, "revaluation_reserves"),
	lineDef("2OCC", "Yabancı Para Çevrim Farkları", "Foreign currency translation differences", StatementBalanceSheet, "foreign_currency_translation_differences"),
	lineDef("2OCD", "Kardan Ayrılan Kısıtlanmış Yedekler", "Restricted reserves", StatementBalanceSheet, "restricted_reserves"),
	lineDef("2OCE", "Geçmiş Yıllar Kar/Zararları", "Retained earnings", StatementBalanceSheet, "retained_earnings"),
	lineDef("2OCF", "Dönem Net Kar/Zararı", "Current period net income", StatementBalanceSheet, "current_period_net_income"),
	lineDef("2OD", "Diğer Özsermaye Kalemleri", "Other equity items", StatementBalanceSheet, "other_equity_items"),
	lineDef("2ODA", "Azınlık Payları", "Minority interests", StatementBalanceSheet, "minority_interests", "Kontrol Gücü Olmayan Paylar"),
	lineDef("2ODB", "TOPLAM KAYNAKLAR", "Total liabilities and equity", StatementBalanceSheet, "total_liabilities_and_equity", "Toplam Yükümlülükler ve Özkaynaklar"),
	lineDef("3B", "Sürdürülen Faaliyetler", "Continuing operations", StatementIncome, "continuing_operations"),
	lineDef("3C", "Satış Gelirleri", "Net sales", StatementIncome, "revenue", "Hasılat", "Satış Geliri"),
	lineDef("3CA", "Satışların Maliyeti (-)", "Cost of sales", StatementIncome, "cost_of_sales", "Satışların Maliyeti"),
	lineDef("3CAA", "Ticari Faaliyetlerden Diğer Kar/Zarar", "Other profit/loss from trade operations", StatementIncome, "other_trade_operations_profit_loss"),
	lineDef("3CAB", "Ticari Faaliyetlerden Brüt Kar/Zarar", "Gross profit/loss from trade operations", StatementIncome, "gross_profit_from_trade_operations"),
	lineDef("3CAC", "Faiz, Ücret, Prim, Komisyon ve Diğer Gelirler", "Interest, fee, premium, commission and other income", StatementIncome, "financial_sector_revenue"),
	lineDef("3CAD", "Faiz, Ücret, Prim, Komisyon ve Diğer Giderler (-)", "Interest, fee, premium, commission and other expenses", StatementIncome, "financial_sector_cost"),
	lineDef("3CAE", "Finans Sektörü Faaliyetlerinden Diğer Kar/Zarar", "Other profit/loss from financial operations", StatementIncome, "other_financial_operations_profit_loss"),
	lineDef("3CAF", "Finans Sektörü Faaliyetlerinden Brüt Kar/Zarar", "Gross profit/loss from financial operations", StatementIncome, "gross_profit_from_financial_operations", "Finans Sektörü Faaliyetlerinden Brüt Kâr/Zarar"),
	lineDef("3CAFA", "Finans Sektörü Faaliyetlerinden Brüt Kâr/Zarar Finansman Gideri Öncesi Faaliyet Kârı/Zararı", "Financial sector gross profit/loss and operating profit/loss before finance expense", StatementIncome, "financial_sector_gross_profit_before_finance_expense"),
	lineDef("3CB", "Diğer Gelir ve Giderler", "Other income and expenses", StatementIncome, "other_income_expenses"),
	lineDef("3D", "BRÜT KAR/ZARAR", "Gross profit/loss", StatementIncome, "gross_profit", "Brüt Kar"),
	lineDef("3DA", "Pazarlama, Satış ve Dağıtım Giderleri (-)", "Marketing, selling and distribution expenses", StatementIncome, "marketing_sales_distribution_expenses", "Pazarlama Giderleri"),
	lineDef("3DB", "Genel Yönetim Giderleri (-)", "General administrative expenses", StatementIncome, "general_administrative_expenses"),
	lineDef("3DC", "Araştırma ve Geliştirme Giderleri (-)", "Research and development expenses", StatementIncome, "research_development_expenses", "Ar-Ge Giderleri"),
	lineDef("3DD", "Diğer Faaliyet Gelirleri", "Other operating income", StatementIncome, "other_operating_income"),
	lineDef("3DE", "Diğer Faaliyet Giderleri (-)", "Other operating expenses", StatementIncome, "other_operating_expenses"),
	lineDef("3DEA", "Faaliyet Karı Öncesi Diğer Gelir ve Giderler", "Other income and expenses before operating profit", StatementIncome, "other_income_expenses_before_operating_profit"),
	lineDef("3DF", "FAALİYET KARI/ZARARI", "Operating profit/loss", StatementIncome, "operating_profit", "Esas Faaliyet Karı", "Esas Faaliyet Karı/Zararı"),
	lineDef("3H", "Net Faaliyet Kar/Zararı", "Net operating profit/loss", StatementIncome, "net_operating_profit"),
	lineDef("3HA", "Yatırım Faaliyetlerinden Gelirler", "Income from investing activities", StatementIncome, "investment_activity_income"),
	lineDef("3HAA", "Yatırım Faaliyetlerinden Giderler (-)", "Expenses from investing activities", StatementIncome, "investment_activity_expenses"),
	lineDef("3HAB", "Diğer Gelir ve Giderler", "Other income and expenses", StatementIncome, "other_non_operating_income_expenses"),
	lineDef("3HAC", "Özkaynak Yöntemiyle Değerlenen Yatırımların Kar/Zararlarındaki Paylar", "Share of profit/loss of equity method investments", StatementIncome, "share_of_profit_loss_from_equity_method_investments"),
	lineDef("3HACA", "Finansman Gideri Öncesi Faaliyet Karı/Zararı", "Operating profit/loss before finance expense", StatementIncome, "operating_profit_before_finance_expense", "Finansman Gideri Öncesi Faaliyet Kârı/Zararı", "Finans Sektörü Faaliyetlerinden Brüt Kar/Zarar Finansman Gideri Öncesi Faaliyet Karı/Zararı", "Finans Sektörü Faaliyetlerinden Brüt Kâr/Zarar Finansman Gideri Öncesi Faaliyet Kârı/Zararı"),
	lineDef("3HB", "Finansman Gelirleri", "Finance income", StatementIncome, "finance_income", "(Esas Faaliyet Dışı) Finansal Gelirler"),
	lineDef("3HC", "Finansman Giderleri (-)", "Finance expenses", StatementIncome, "finance_expenses", "finance_expense", "(Esas Faaliyet Dışı) Finansal Giderler"),
	lineDef("3HCA", "Vergi Öncesi Diğer Gelir ve Giderler", "Other income and expenses before tax", StatementIncome, "other_income_expenses_before_tax"),
	lineDef("3I", "SÜRDÜRÜLEN FAALİYETLER VERGİ ÖNCESİ KARI/ZARARI", "Profit/loss before tax from continuing operations", StatementIncome, "profit_before_tax", "Vergi Öncesi Kar/Zarar"),
	lineDef("3IA", "Sürdürülen Faaliyetler Vergi Geliri/Gideri", "Tax income/expense on continuing operations", StatementIncome, "continuing_operations_tax_income_expense"),
	lineDef("3IB", "Dönem Vergi Geliri/Gideri", "Current tax income/expense", StatementIncome, "current_tax_income_expense"),
	lineDef("3IC", "Ertelenmiş Vergi Geliri/Gideri", "Deferred tax income/expense", StatementIncome, "deferred_tax_income_expense"),
	lineDef("3ID", "Diğer Vergi Geliri/Gideri", "Other tax income/expense", StatementIncome, "other_tax_income_expense"),
	lineDef("3J", "SÜRDÜRÜLEN FAALİYETLER DÖNEM KARI/ZARARI", "Profit/loss from continuing operations", StatementIncome, "continuing_operations_net_income"),
	lineDef("3K", "DURDURULAN FAALİYETLER", "Discontinued operations", StatementIncome, "discontinued_operations"),
	lineDef("3KA", "Durdurulan Faaliyetler Vergi Sonrası Dönem Karı/Zararı", "Profit/loss after tax from discontinued operations", StatementIncome, "discontinued_operations_net_income"),
	lineDef("3L", "DÖNEM KARI/ZARARI", "Net income/loss", StatementIncome, "net_income", "Net Dönem Karı", "Net Dönem Karı/Zararı"),
	lineDef("3LA", "Dönem Kar/Zararının Dağılımı", "Distribution of profit/loss", StatementIncome, "profit_loss_distribution"),
	lineDef("3LB", "Azınlık Payları", "Minority interests", StatementIncome, "minority_net_income"),
	lineDef("3Z", "Ana Ortaklık Payları", "Parent shares", StatementIncome, "parent_net_income"),
	lineDef("3ZD", "Hisse Başına Kazanç", "Earnings per share", StatementIncome, "earnings_per_share", "Pay Başına Kazanç"),
	lineDef("3ZE", "Seyreltilmiş Hisse Başına Kazanç", "Diluted earnings per share", StatementIncome, "diluted_earnings_per_share"),
	lineDef("3ZF", "Sürdürülen Faaliyetlerden Hisse Başına Kazanç", "Earnings per share from continuing operations", StatementIncome, "continuing_operations_earnings_per_share"),
	lineDef("3ZG", "Sürdürülen Faaliyetlerden Seyreltilmiş Hisse Başına Kazanç", "Diluted earnings per share from continuing operations", StatementIncome, "continuing_operations_diluted_earnings_per_share"),
	lineDef("4B", "Amortisman Giderleri", "Depreciation and amortization", StatementCashFlow, "depreciation_amortization"),
	lineDef("4BA", "Kıdem Tazminatı", "Severance payments", StatementCashFlow, "severance_payments"),
	lineDef("4BB", "Finansman Giderleri", "Financial expenses", StatementCashFlow, "cash_flow_financial_expenses"),
	lineDef("4BC", "Yurtiçi Satışlar", "Domestic sales", StatementIncome, "domestic_sales"),
	lineDef("4BD", "Yurtdışı Satışlar", "Export sales", StatementIncome, "export_sales"),
	lineDef("4BE", "Net Yabancı Para Pozisyonu", "Net foreign currency position", StatementBalanceSheet, "net_foreign_currency_position"),
	lineDef("4BEA", "Parasal Net Yabancı Para Varlık/Yükümlülük Pozisyonu", "Monetary net foreign currency asset/liability position", StatementBalanceSheet, "monetary_net_foreign_currency_position"),
	lineDef("4BEB", "Net YPP Hedge Dahil", "Net foreign currency position including hedge", StatementBalanceSheet, "net_foreign_currency_position_including_hedge"),
	lineDef("4C", "İşletme Faaliyetlerinden Kaynaklanan Net Nakit", "Net cash from operations", StatementCashFlow, "operating_cash_flow", "İşletme Faaliyetlerinden Nakit Akışları", "İşletme Faaliyetlerinden Kaynaklanan Nakit Akışları", "İşletme Faaliyetlerden Nakit Akışları"),
	lineDef("4CA", "Düzeltme Öncesi Kar", "Earnings before adjustments", StatementCashFlow, "earnings_before_adjustments", "Dönem Karı/Zararı"),
	lineDef("4CAA", "Düzeltmeler", "Adjustments", StatementCashFlow, "cash_flow_adjustments"),
	lineDef("4CAB", "Amortisman ve İtfa Payları", "Depreciation and amortisation", StatementCashFlow, "depreciation_amortisation_adjustments", "Amortisman ve İtfa Gideri ile İlgili Düzeltmeler"),
	lineDef("4CAC", "Karşılıklardaki Değişim", "Change in provisions", StatementCashFlow, "change_in_provisions"),
	lineDef("4CAD", "Diğer Gelir/Gider", "Other income/expense", StatementCashFlow, "cash_flow_other_income_expense"),
	lineDef("4CAE", "İşletme Sermayesinde Değişiklikler Öncesi Faaliyet Karı", "Operating profit before working capital changes", StatementCashFlow, "operating_profit_before_working_capital_changes"),
	lineDef("4CAF", "İşletme Sermayesindeki Değişiklikler", "Change in working capital", StatementCashFlow, "change_in_working_capital", "İşletme Sermayesinde Gerçekleşen Değişimler"),
	lineDef("4CAG", "Esas Faaliyet ile İlgili Oluşan Nakit", "Cash from core operations", StatementCashFlow, "cash_from_core_operations", "Faaliyetlerden Elde Edilen Nakit Akışları", "Faaliyetlerden Net Nakit Akışları"),
	lineDef("4CAH", "Diğer İşletme Faaliyetlerinden Nakit", "Cash from other operating activities", StatementCashFlow, "cash_from_other_operating_activities"),
	lineDef("4CAI", "Sabit Sermaye Yatırımları", "Capital expenditures", StatementCashFlow, "capital_expenditures", "CapEx", "Maddi ve Maddi Olmayan Duran Varlıkların Alımından Kaynaklanan Nakit Çıkışları"),
	lineDef("4CAJ", "Diğer Yatırım Faaliyetlerinden Nakit", "Cash from other investing activities", StatementCashFlow, "cash_from_other_investing_activities"),
	lineDef("4CAK", "Yatırım Faaliyetlerinden Kaynaklanan Nakit", "Cash from investing activities", StatementCashFlow, "investing_cash_flow", "Yatırım Faaliyetlerinden Kaynaklanan Nakit Akışları"),
	lineDef("4CB", "Serbest Nakit Akım", "Free cash flow", StatementCashFlow, "free_cash_flow"),
	lineDef("4CBA", "Finansal Borçlardaki Değişim", "Change in financial debt", StatementCashFlow, "change_in_financial_debt", "Borçlanmadan Kaynaklanan Nakit Girişleri", "Borç Ödemelerine İlişkin Nakit Çıkışları"),
	lineDef("4CBB", "Temettü Ödemeleri", "Dividends paid", StatementCashFlow, "dividends_paid", "Ödenen Temettüler"),
	lineDef("4CBC", "Sermaye Artırımı", "Rights issue", StatementCashFlow, "rights_issue", "Pay ve Diğer Özkaynağa Dayalı Araçların İhracından Kaynaklanan Nakit Girişleri"),
	lineDef("4CBD", "Diğer Finansman Faaliyetlerinden Nakit", "Cash from other financing activities", StatementCashFlow, "cash_from_other_financing_activities"),
	lineDef("4CBE", "Finansman Faaliyetlerinden Kaynaklanan Nakit", "Cash from financing activities", StatementCashFlow, "financing_cash_flow", "Finansman Faaliyetlerinden Kaynaklanan Nakit Akışları"),
	lineDef("4CBF", "Yabancı Para Çevrim Farkları Etkisi Öncesi Nakit ve Nakit Benzerleri Net Artış/Azalış", "Net increase/decrease in cash before foreign currency translation effects", StatementCashFlow, "cash_change_before_fx_effect", "Yabancı Para Çevirim Farklarının Etkisinden Önce Nakit ve Nakit Benzerlerindeki Net Artış/Azalış"),
	lineDef("4CBG", "Yabancı Para Çevrim Farklarının Nakit ve Nakit Benzerleri Üzerindeki Etkisi", "Effect of foreign currency translation on cash and cash equivalents", StatementCashFlow, "fx_translation_effect_on_cash"),
	lineDef("4CBH", "Diğer Nakit Girişi/Çıkışı", "Other cash inflow/outflow", StatementCashFlow, "other_cash_inflow_outflow"),
	lineDef("4CBI", "Nakit ve Benzerlerindeki Değişim", "Change in cash and cash equivalents", StatementCashFlow, "change_in_cash_and_cash_equivalents", "Nakit ve Nakit Benzerlerindeki Net Artış/Azalış"),
	lineDef("4CBJ", "Diğer Nakit ve Nakit Benzerlerindeki Artış", "Other cash and cash equivalents change", StatementCashFlow, "other_cash_equivalents_change"),
	lineDef("4CBK", "Dönem Başı Nakit Değerler", "Cash at the beginning of the period", StatementCashFlow, "cash_at_beginning_of_period", "Dönem Başı Nakit ve Nakit Benzerleri"),
	lineDef("4CBL", "Dönem Sonu Nakit", "Cash at the end of the period", StatementCashFlow, "cash_at_end_of_period", "Dönem Sonu Nakit ve Nakit Benzerleri"),

	// SPK/KGK format guide fields that are not present in the legacy bilanco schema,
	// but should still be recognized by the PDF processor as financial table lines.
	lineDef("", "Finansal Tablo ve Dipnot Formatları Hakkında Duyuru", "Announcement on financial statement and note formats", StatementNote, "financial_statement_note_formats_announcement"),
	lineDef("", "Finansal Durum Tablosu", "Statement of financial position", StatementBalanceSheet, "financial_position_statement", "Bilanço"),
	lineDef("", "Kar veya Zarar ve Diğer Kapsamlı Gelir Tablosu", "Statement of profit or loss and other comprehensive income", StatementIncome, "profit_or_loss_and_oci_statement"),
	lineDef("", "Kar veya Zarar Tablosu", "Statement of profit or loss", StatementIncome, "profit_or_loss_statement"),
	lineDef("", "Kar veya Zarar Kısmı", "Profit or loss section", StatementIncome, "profit_or_loss_section"),
	lineDef("", "Diğer Kapsamlı Gelir Tablosu", "Statement of other comprehensive income", StatementIncome, "other_comprehensive_income_statement"),
	lineDef("", "Diğer Kapsamlı Gelirler", "Other comprehensive income items", StatementIncome, "other_comprehensive_income_items"),
	lineDef("", "Özkaynaklar Değişim Tablosu", "Statement of changes in equity", StatementEquity, "statement_of_changes_in_equity"),
	lineDef("", "Nakit Akış Tablosu", "Statement of cash flows", StatementCashFlow, "statement_of_cash_flows"),
	lineDef("", "Dipnot Referansları", "Note references", StatementNote, "note_references"),
	lineDef("", "Geçmiş Dönemi", "Prior period", StatementNote, "past_period_column"),
	lineDef("", "Ara Toplam", "Subtotal", StatementNote, "subtotal"),
	lineDef("", "Toplam Dönen Varlıklar", "Total current assets", StatementBalanceSheet, "total_current_assets"),
	lineDef("", "Toplam Duran Varlıklar", "Total non-current assets", StatementBalanceSheet, "total_non_current_assets"),
	lineDef("", "Toplam Kısa Vadeli Yükümlülükler", "Total current liabilities", StatementBalanceSheet, "total_current_liabilities"),
	lineDef("", "Toplam Uzun Vadeli Yükümlülükler", "Total non-current liabilities", StatementBalanceSheet, "total_non_current_liabilities"),
	lineDef("", "Toplam Kaynaklar", "Total liabilities and equity", StatementBalanceSheet, "total_sources"),
	lineDef("", "Diğer Maddi Olmayan Duran Varlıklar", "Other intangible assets", StatementBalanceSheet, "other_intangible_assets"),
	lineDef("", "Peşin Ödenmiş Giderler", "Prepaid expenses", StatementBalanceSheet, "prepaid_expenses"),
	lineDef("", "Cari Dönem Vergisiyle İlgili Varlıklar", "Current tax assets", StatementBalanceSheet, "current_tax_assets"),
	lineDef("", "Türev Araçlar", "Derivative instruments", StatementBalanceSheet, "derivative_instruments"),
	lineDef("", "Sermaye Düzeltme Farkları", "Capital adjustment differences", StatementBalanceSheet, "capital_adjustment_differences"),
	lineDef("", "Geri Alınmış Paylar (-)", "Treasury shares", StatementBalanceSheet, "treasury_shares"),
	lineDef("", "Toplam Kapsamlı Gelir", "Total comprehensive income", StatementIncome, "total_comprehensive_income"),
	lineDef("", "Satılan Mallardan ve Hizmetlerden Elde Edilen Nakit Girişleri", "Cash receipts from sales of goods and services", StatementCashFlow, "cash_receipts_from_sales"),
	lineDef("", "Mal ve Hizmetler İçin Tedarikçilere Yapılan Ödemeler", "Cash payments to suppliers for goods and services", StatementCashFlow, "cash_payments_to_suppliers"),
	lineDef("", "Çalışanlara ve Çalışanlar Adına Yapılan Ödemelerden Kaynaklanan Nakit Çıkışları", "Cash payments to and on behalf of employees", StatementCashFlow, "cash_payments_to_employees"),
	lineDef("", "Vergi Ödemeleri/İadeleri", "Tax payments/refunds", StatementCashFlow, "tax_payments_refunds", "Vergi Ödemeleri/İadeler", "Vergi Ödemeler/İadeler"),
	lineDef("", "Diğer Nakit Girişleri/Çıkışları", "Other cash inflows/outflows", StatementCashFlow, "other_cash_inflows_outflows"),
	lineDef("", "Alınan Faiz", "Interest received", StatementCashFlow, "interest_received"),
	lineDef("", "Ödenen Faiz", "Interest paid", StatementCashFlow, "interest_paid"),
	lineDef("", "Alınan Temettüler", "Dividends received", StatementCashFlow, "dividends_received"),
	lineDef("", "Dönem Net Karı/Zararı Mutabakatı ile İlgili Düzeltmeler", "Adjustments to reconcile net income/loss", StatementCashFlow, "net_income_reconciliation_adjustments"),
	lineDef("", "İlişkili Taraflardan Ticari Alacaklar", "Trade receivables from related parties", StatementBalanceSheet, "related_party_trade_receivables"),
	lineDef("", "İlişkili Olmayan Taraflardan Ticari Alacaklar", "Trade receivables from third parties", StatementBalanceSheet, "third_party_trade_receivables"),
	lineDef("", "Finans Sektörü Faaliyetleri İlişkili Taraflardan Alacaklar", "Financial sector receivables from related parties", StatementBalanceSheet, "related_party_financial_operations_receivables"),
	lineDef("", "Finans Sektörü Faaliyetlerinden İlişkili Olmayan Taraflardan Alacaklar", "Financial sector receivables from third parties", StatementBalanceSheet, "third_party_financial_operations_receivables"),
	lineDef("", "İlişkili Taraflardan Diğer Alacaklar", "Other receivables from related parties", StatementBalanceSheet, "related_party_other_receivables"),
	lineDef("", "İlişkili Olmayan Taraflardan Diğer Alacaklar", "Other receivables from third parties", StatementBalanceSheet, "third_party_other_receivables"),
	lineDef("", "İlişkili Taraflara Ticari Borçlar", "Trade payables to related parties", StatementBalanceSheet, "related_party_trade_payables"),
	lineDef("", "İlişkili Olmayan Taraflara Ticari Borçlar", "Trade payables to third parties", StatementBalanceSheet, "third_party_trade_payables", "İlişkili Taraflara Olmayan Ticari Borçlar"),
	lineDef("", "Finans Sektörü Faaliyetleri İlişkili Taraflara Borçlar", "Financial sector payables to related parties", StatementBalanceSheet, "related_party_financial_operations_payables"),
	lineDef("", "Finans Sektörü Faaliyetlerinden Borçlar", "Payables from financial sector operations", StatementBalanceSheet, "financial_operations_payables"),
	lineDef("", "Finans Sektörü Faaliyetlerinden İlişkili Olmayan Taraflara Borçlar", "Financial sector payables to third parties", StatementBalanceSheet, "third_party_financial_operations_payables"),
	lineDef("", "Çalışanlara Sağlanan Faydalar Kapsamında Borçlar", "Payables related to employee benefits", StatementBalanceSheet, "employee_benefit_payables"),
	lineDef("", "İlişkili Taraflara Diğer Borçlar", "Other payables to related parties", StatementBalanceSheet, "related_party_other_payables"),
	lineDef("", "İlişkili Olmayan Taraflara Diğer Borçlar", "Other payables to third parties", StatementBalanceSheet, "third_party_other_payables"),
	lineDef("", "Diğer Borçlar", "Other payables", StatementBalanceSheet, "other_payables"),
	lineDef("", "Devlet Teşvik ve Yardımları", "Government grants", StatementBalanceSheet, "government_grants"),
	lineDef("", "Ertelenmiş Gelirler", "Deferred income", StatementBalanceSheet, "deferred_income"),
	lineDef("", "Çalışanlara Sağlanan Faydalara İlişkin Kısa Vadeli Karşılıklar", "Short-term employee benefit provisions", StatementBalanceSheet, "short_term_employee_benefit_provisions"),
	lineDef("", "Diğer Kısa Vadeli Karşılıklar", "Other short-term provisions", StatementBalanceSheet, "other_short_term_provisions"),
	lineDef("", "Çalışanlara Sağlanan Faydalara İlişkin Uzun Vadeli Karşılıklar", "Long-term employee benefit provisions", StatementBalanceSheet, "long_term_employee_benefit_provisions"),
	lineDef("", "Diğer Uzun Vadeli Karşılıklar", "Other long-term provisions", StatementBalanceSheet, "other_long_term_provisions"),
	lineDef("", "Cari Dönem Vergisiyle İlgili Borçlar", "Current tax payables", StatementBalanceSheet, "current_tax_payables"),
	lineDef("", "Satış Amaçlı Sınıflandırılan Varlık Gruplarına İlişkin Yükümlülükler", "Liabilities related to disposal groups classified as held for sale", StatementBalanceSheet, "liabilities_related_to_disposal_groups_held_for_sale"),
	lineDef("", "Paylara İlişkin Primler/İskontolar", "Share premiums/discounts", StatementBalanceSheet, "share_premiums_discounts"),
	lineDef("", "Kar veya Zararda Yeniden Sınıflandırılmayacak Birikmiş Diğer Kapsamlı Gelirler veya Giderler", "Accumulated other comprehensive income/loss not to be reclassified to profit or loss", StatementBalanceSheet, "oci_not_reclassified_reserves"),
	lineDef("", "Yeniden Değerleme ve Ölçüm Kazanç/Kayıpları", "Revaluation and measurement gains/losses", StatementBalanceSheet, "revaluation_measurement_gains_losses"),
	lineDef("", "Diğer Kazanç/Kayıplar", "Other gains/losses", StatementBalanceSheet, "other_gains_losses"),
	lineDef("", "Karşılıklı İştirak Sermaye Düzeltmesi", "Reciprocal shareholding capital adjustment", StatementBalanceSheet, "reciprocal_shareholding_capital_adjustment"),
	lineDef("", "Yabancı Para Çevirim Farkları", "Foreign currency translation differences", StatementBalanceSheet, "foreign_currency_translation_differences_alt"),
	lineDef("", "Geçmiş Yıllar Karları/Zararları", "Prior years' profits/losses", StatementBalanceSheet, "prior_years_profit_loss"),
	lineDef("2N", "Toplam Özkaynaklar", "Total equity", StatementBalanceSheet, "equity"),
	lineDef("", "Kar veya Zararda Yeniden Sınıflandırılacak Birikmiş Diğer Kapsamlı Gelirler veya Giderler", "Accumulated other comprehensive income/loss to be reclassified to profit or loss", StatementBalanceSheet, "oci_reclassified_reserves"),
	lineDef("", "Riskten Korunma Kazanç/Kayıpları", "Hedging gains/losses", StatementBalanceSheet, "hedging_gains_losses"),
	lineDef("", "Yeniden Değerleme ve Sınıflandırma Kazanç/Kayıpları", "Revaluation and reclassification gains/losses", StatementBalanceSheet, "revaluation_reclassification_gains_losses"),
	lineDef("", "Finans Sektörü Faaliyetleri Hasılatı", "Financial sector operating revenue", StatementIncome, "financial_sector_operating_revenue"),
	lineDef("", "Finans Sektörü Faaliyetleri Maliyeti", "Financial sector operating cost", StatementIncome, "financial_sector_operating_cost"),
	lineDef("", "Esas Faaliyetlerden Diğer Gelirler", "Other operating income", StatementIncome, "other_operating_income_from_core_activities"),
	lineDef("", "Esas Faaliyetlerden Diğer Giderler", "Other operating expenses", StatementIncome, "other_operating_expenses_from_core_activities"),
	lineDef("", "Özkaynak Yöntemiyle Değerlenen Yatırımların Karlarından/Zararlarından Paylar", "Share of profit/loss from equity-accounted investments", StatementIncome, "share_of_profit_loss_from_equity_method_investments"),
	lineDef("", "Sürdürülen Faaliyetler Vergi Gideri/Geliri", "Tax expense/income from continuing operations", StatementIncome, "continuing_operations_tax_expense_income"),
	lineDef("", "Dönem Vergi Gideri/Geliri", "Current tax expense/income", StatementIncome, "current_tax_expense_income"),
	lineDef("", "Ertelenmiş Vergi Gideri/Geliri", "Deferred tax expense/income", StatementIncome, "deferred_tax_expense_income"),
	lineDef("", "Durdurulan Faaliyetler Dönem Karı/Zararı", "Net income/loss from discontinued operations", StatementIncome, "discontinued_operations_period_profit_loss"),
	lineDef("", "Durdurulan Faaliyetlerden Gelirler", "Income from discontinued operations", StatementIncome, "discontinued_operations_income"),
	lineDef("", "Durdurulan Faaliyetlerden Giderler (-)", "Expenses from discontinued operations", StatementIncome, "discontinued_operations_expenses", "Durdurulan Faaliyetlerden Giderler"),
	lineDef("", "Durdurulan faaliyetler vergi öncesi kârı/zararı", "Profit/loss before tax from discontinued operations", StatementIncome, "discontinued_operations_profit_before_tax"),
	lineDef("", "Durdurulan Faaliyetler Vergi Gideri/Geliri", "Tax expense/income from discontinued operations", StatementIncome, "discontinued_operations_tax_expense_income"),
	lineDef("", "Sürdürülen Faaliyetlerden Pay Başına Kazanç", "Earnings per share from continuing operations", StatementIncome, "continuing_operations_eps"),
	lineDef("", "Durdurulan Faaliyetlerden Pay Başına Kazanç", "Earnings per share from discontinued operations", StatementIncome, "discontinued_operations_eps"),
	lineDef("", "Sulandırılmış Pay Başına Kazanç", "Diluted earnings per share", StatementIncome, "diluted_eps", "Seyreltilmiş Pay Başına Kazanç"),
	lineDef("", "Sürdürülen Faaliyetlerden Sulandırılmış Pay Başına Kazanç", "Diluted earnings per share from continuing operations", StatementIncome, "continuing_operations_diluted_eps"),
	lineDef("", "Durdurulan Faaliyetlerden Sulandırılmış Pay Başına Kazanç", "Diluted earnings per share from discontinued operations", StatementIncome, "discontinued_operations_diluted_eps"),
	lineDef("", "Kar veya Zararda Yeniden Sınıflandırılmayacaklar", "Items not to be reclassified to profit or loss", StatementIncome, "oci_items_not_reclassified"),
	lineDef("", "Maddi Duran Varlıklar Yeniden Değerleme Artışları/Azalışları", "PPE revaluation gains/losses", StatementIncome, "ppe_revaluation_gains_losses"),
	lineDef("", "Maddi Olmayan Duran Varlıklar Yeniden Değerleme Artışları/Azalışları", "Intangible asset revaluation gains/losses", StatementIncome, "intangible_revaluation_gains_losses"),
	lineDef("", "Tanımlanmış Fayda Planları Yeniden Ölçüm Kazançları/Kayıpları", "Remeasurement gains/losses on defined benefit plans", StatementIncome, "defined_benefit_remeasurement_gains_losses"),
	lineDef("", "Özkaynak Yöntemiyle Değerlenen Yatırımların Diğer Kapsamlı Gelirinden Kar/Zararda Sınıflandırılmayacak Paylar", "Share of OCI from equity method investments not reclassified", StatementIncome, "equity_method_oci_not_reclassified_share"),
	lineDef("", "Diğer Kar veya Zarar Olarak Yeniden Sınıflandırılmayacak Diğer Kapsamlı Gelir Unsurları", "Other OCI items not reclassified", StatementIncome, "other_oci_not_reclassified_items"),
	lineDef("", "Kar veya Zararda Yeniden Sınıflandırılmayacak Diğer Kapsamlı Gelire İlişkin Vergiler", "Taxes related to OCI items not reclassified", StatementIncome, "taxes_on_oci_not_reclassified"),
	lineDef("", "Kar veya Zarar Olarak Yeniden Sınıflandırılacaklar", "Items to be reclassified to profit or loss", StatementIncome, "oci_items_reclassified"),
	lineDef("", "Satılmaya Hazır Finansal Varlıkların Yeniden Değerleme ve/veya Sınıflandırma Kazançları/Kayıpları", "Available-for-sale financial asset revaluation/reclassification gains/losses", StatementIncome, "afs_revaluation_reclassification_gains_losses"),
	lineDef("", "Nakit Akış Riskinden Korunma Kazançları/Kayıpları", "Cash flow hedge gains/losses", StatementIncome, "cash_flow_hedge_gains_losses"),
	lineDef("", "Yurtdışındaki İşletmeye İlişkin Yatırım Riskinden Korunma Kazançları/Kayıpları", "Hedge of net investment in foreign operation gains/losses", StatementIncome, "foreign_operation_investment_hedge_gains_losses"),
	lineDef("", "Özkaynak Yöntemiyle Değerlenen Yatırımların Diğer Kapsamlı Gelirinden Kar/Zararda Sınıflandırılacak Paylar", "Share of OCI from equity method investments reclassified", StatementIncome, "equity_method_oci_reclassified_share"),
	lineDef("", "Diğer Kar veya Zarar Olarak Yeniden Sınıflandırılacak Diğer Kapsamlı Gelir Unsurları", "Other OCI items reclassified", StatementIncome, "other_oci_reclassified_items"),
	lineDef("", "Kar veya Zararda Yeniden Sınıflandırılacak Diğer Kapsamlı Gelire İlişkin Vergiler Gelir/Giderler", "Taxes related to OCI items reclassified", StatementIncome, "taxes_on_oci_reclassified"),
	lineDef("", "Diğer Kapsamlı Gelir", "Other comprehensive income", StatementIncome, "other_comprehensive_income"),
	lineDef("", "Toplam Kapsamlı Gelirin Dağılımı", "Distribution of total comprehensive income", StatementIncome, "total_comprehensive_income_distribution"),
	lineDef("", "Muhasebe Politikalarındaki Değişikliklere İlişkin Düzeltmeler", "Adjustments related to changes in accounting policies", StatementEquity, "accounting_policy_change_adjustments"),
	lineDef("", "Kar veya Zararda Yeniden Sınıflandırılmayacak Birikmiş Diğer Kapsamlı Gelirler ve Giderler", "Accumulated OCI not to be reclassified to profit or loss", StatementEquity, "equity_oci_not_reclassified_column"),
	lineDef("", "Kar veya Zararda Yeniden Sınıflandırılacak Birikmiş Diğer Kapsamlı Gelirler ve Giderler", "Accumulated OCI to be reclassified to profit or loss", StatementEquity, "equity_oci_reclassified_column"),
	lineDef("", "Birikmiş Karlar", "Accumulated profits", StatementEquity, "accumulated_profits_column"),
	lineDef("", "Pay İhraç Primleri/İskontoları", "Share issue premiums/discounts", StatementEquity, "share_issue_premiums_discounts"),
	lineDef("", "Yeniden Değerleme ve Ölçüm Kazanç / Kayıpları", "Revaluation and measurement gains/losses", StatementEquity, "equity_revaluation_measurement_gains_losses"),
	lineDef("", "Riskten Korunma Kazanç / Kayıpları", "Hedging gains/losses", StatementEquity, "equity_hedging_gains_losses"),
	lineDef("", "Yeniden Değerleme ve Sınıflandırma Kazanç / Kayıpları", "Revaluation and reclassification gains/losses", StatementEquity, "equity_revaluation_reclassification_gains_losses"),
	lineDef("", "Geçmiş Yıllar Kar / Zararları", "Prior years profits/losses", StatementEquity, "equity_prior_years_profit_loss"),
	lineDef("", "Net Dönem Karı Zararı", "Current period profit/loss", StatementEquity, "equity_current_period_profit_loss", "Net Dönem Karı / Zararı"),
	lineDef("", "Hatalara İlişkin Düzeltmeler", "Error corrections", StatementEquity, "error_corrections"),
	lineDef("", "Transferler", "Transfers", StatementEquity, "equity_transfers"),
	lineDef("", "Sermaye Artırımı", "Capital increase", StatementEquity, "capital_increase"),
	lineDef("", "Temettüler", "Dividends", StatementEquity, "dividends"),
	lineDef("", "Payların Geri Alım İşlemleri Nedeniyle Meydana Gelen Artış/Azalış", "Increase/decrease due to treasury share transactions", StatementEquity, "treasury_share_transaction_change"),
	lineDef("", "Pay Bazlı İşlemler Nedeniyle Meydana Gelen Artış", "Increase due to share-based transactions", StatementEquity, "share_based_transaction_increase"),
	lineDef("", "Bağlı Ortaklıklarda Kontrol Kaybı ile Sonuçlanmayan Pay Oranı Değişikliklerine Bağlı Artış/Azalış", "Change due to ownership changes in subsidiaries without loss of control", StatementEquity, "subsidiary_ownership_change_without_loss_of_control"),
	lineDef("", "Kontrol Gücü Olmayan Pay Sahipleri ile Yapılan İşlemler", "Transactions with non-controlling interests", StatementEquity, "transactions_with_non_controlling_interests"),
	lineDef("", "Diğer Değişiklikler Nedeniyle Artış/Azalış", "Increase/decrease due to other changes", StatementEquity, "other_equity_changes"),
	lineDef("", "Dönem Başı Bakiyeler", "Opening balances", StatementEquity, "equity_opening_balances", "itibariyle bakiyeler Dönem Başı"),
	lineDef("", "Dönem Sonu Bakiyeler", "Closing balances", StatementEquity, "equity_closing_balances", "itibariyle bakiyeler Dönem Sonu"),
	lineDef("", "İşletme Faaliyetlerinden Kaynaklanan Nakit Girişi Sınıfları", "Classes of cash inflows from operating activities", StatementCashFlow, "operating_cash_inflow_classes"),
	lineDef("", "Faiz, Ücret, Prim, Komisyon ve Diğer Gelirlerden Nakit Girişleri", "Cash inflows from interest, fee, premium, commission and other income", StatementCashFlow, "cash_inflows_from_interest_fee_premium_commission_other"),
	lineDef("", "Alım Satım Amaçlı Elde Bulundurulan Sözleşmeler ile İlgili Nakit Girişleri", "Cash inflows from held-for-trading contracts", StatementCashFlow, "cash_inflows_from_held_for_trading_contracts"),
	lineDef("", "İşletme Faaliyetlerinden Kaynaklanan Diğer Nakit Girişleri", "Other cash inflows from operating activities", StatementCashFlow, "other_operating_cash_inflows"),
	lineDef("", "İşletme Faaliyetlerinden Nakit Çıkışları", "Cash outflows from operating activities", StatementCashFlow, "operating_cash_outflows"),
	lineDef("", "Faiz, Ücret, Prim, Komisyon ve Diğer Gelirlerden Nakit Çıkışları", "Cash outflows from interest, fee, premium, commission and other income", StatementCashFlow, "cash_outflows_from_interest_fee_premium_commission_other"),
	lineDef("", "Alım Satım Amaçlı Elde Bulundurulan Sözleşmelerle İlgili Nakit Çıkışları", "Cash outflows from held-for-trading contracts", StatementCashFlow, "cash_outflows_from_held_for_trading_contracts"),
	lineDef("", "İşletme Faaliyetlerinden Kaynaklanan Diğer Nakit Çıkışları", "Other cash outflows from operating activities", StatementCashFlow, "other_operating_cash_outflows"),
	lineDef("", "Bağlı Ortaklıkların Kontrolünün Kaybı Sonucunu Doğuracak Satışlara İlişkin Nakit Girişleri", "Cash inflows from disposals resulting in loss of control of subsidiaries", StatementCashFlow, "cash_inflows_from_subsidiary_disposals_loss_of_control"),
	lineDef("", "Bağlı Ortaklıkların Kontrolünün Elde Edilmesine Yönelik Alışlara İlişkin Nakit Çıkışları", "Cash outflows for acquisitions resulting in control of subsidiaries", StatementCashFlow, "cash_outflows_for_subsidiary_acquisitions"),
	lineDef("", "Başka İşletmelerin veya Fonların Paylarının veya Borçlanma Araçlarının Satılması Sonucu Elde Edilen Nakit Girişleri", "Cash inflows from sale of other entities' shares or debt instruments", StatementCashFlow, "cash_inflows_from_sale_of_shares_or_debt_instruments"),
	lineDef("", "Başka İşletmelerin veya Fonların Paylarının veya Borçlanma Araçlarının Edinimi İçin Yapılan Nakit Çıkışları", "Cash outflows for acquisition of other entities' shares or debt instruments", StatementCashFlow, "cash_outflows_for_acquisition_of_shares_or_debt_instruments"),
	lineDef("", "Maddi ve Maddi Olmayan Duran Varlıkların Satışından Kaynaklanan Nakit Girişleri", "Cash inflows from sale of PPE and intangible assets", StatementCashFlow, "cash_inflows_from_sale_of_ppe_and_intangibles"),
	lineDef("", "Diğer Uzun Vadeli Varlıkların Satışından Kaynaklanan Nakit Girişleri", "Cash inflows from sale of other long-term assets", StatementCashFlow, "cash_inflows_from_sale_of_other_long_term_assets"),
	lineDef("", "Diğer Uzun Vadeli Varlık Alımlarından Nakit Çıkışları", "Cash outflows for purchase of other long-term assets", StatementCashFlow, "cash_outflows_for_purchase_of_other_long_term_assets"),
	lineDef("", "Verilen Nakit Avans ve Borçlarlar", "Cash advances and loans given", StatementCashFlow, "cash_advances_and_loans_given"),
	lineDef("", "Verilen Nakit Avans ve Borçlardan Geri Ödemeler", "Repayments of cash advances and loans given", StatementCashFlow, "repayments_of_cash_advances_and_loans_given"),
	lineDef("", "Türev Araçlardan Nakit Çıkışları", "Cash outflows from derivative instruments", StatementCashFlow, "cash_outflows_from_derivatives"),
	lineDef("", "Türev Araçlardan Nakit Girişleri", "Cash inflows from derivative instruments", StatementCashFlow, "cash_inflows_from_derivatives"),
	lineDef("", "Devlet Teşviklerinden Elde Edilen Nakit Girişleri", "Cash inflows from government grants", StatementCashFlow, "cash_inflows_from_government_grants"),
	lineDef("", "Pay ve Diğer Özkaynağa Dayalı Araçların İhracından Kaynaklanan Nakit Girişleri", "Cash inflows from issue of shares and other equity instruments", StatementCashFlow, "cash_inflows_from_issue_of_equity_instruments"),
	lineDef("", "İşletmenin Kendi Paylarını ve Diğer Özkaynağa Dayalı Araçlarını Almasıyla İlgili Nakit Çıkışları", "Cash outflows to acquire own shares and other equity instruments", StatementCashFlow, "cash_outflows_to_acquire_own_equity_instruments"),
	lineDef("", "Borçlanmadan Kaynaklanan Nakit Girişleri", "Cash inflows from borrowings", StatementCashFlow, "cash_inflows_from_borrowings"),
	lineDef("", "Borç Ödemelerine İlişkin Nakit Çıkışları", "Cash outflows for debt repayments", StatementCashFlow, "cash_outflows_for_debt_repayments"),
	lineDef("", "Finansal Kiralama Sözleşmelerinden Kaynaklanan Borç Ödemelerine İlişkin Nakit Çıkışları", "Cash outflows for finance lease debt repayments", StatementCashFlow, "cash_outflows_for_finance_lease_debt_repayments"),
	lineDef("", "Yabancı Para Çevrim Farklarının Nakit ve Nakit Benzerleri Üzerindeki Etkisi", "Effect of foreign exchange differences on cash and cash equivalents", StatementCashFlow, "fx_effect_on_cash_and_cash_equivalents"),
	lineDef("", "Değer Düşüklüğü/İptali ile İlgili Düzeltmeler", "Impairment/reversal adjustments", StatementCashFlow, "impairment_reversal_adjustments"),
	lineDef("", "Karşılıklar ile İlgili Düzeltmeler", "Provision adjustments", StatementCashFlow, "provision_adjustments"),
	lineDef("", "Faiz Gelirleri ve Giderleri ile İlgili Düzeltmeler", "Interest income and expense adjustments", StatementCashFlow, "interest_income_expense_adjustments"),
	lineDef("", "Gerçekleşmemiş Yabancı Para Çevirim Farkları ile İlgili Düzeltmeler", "Unrealized foreign currency translation adjustments", StatementCashFlow, "unrealized_fx_translation_adjustments"),
	lineDef("", "Pay Bazlı Ödemeler ile İlgili Düzeltmeler", "Share-based payment adjustments", StatementCashFlow, "share_based_payment_adjustments"),
	lineDef("", "Gerçeğe Uygun Değer Kayıpları/Kazançları ile İlgili Düzeltmeler", "Fair value loss/gain adjustments", StatementCashFlow, "fair_value_loss_gain_adjustments"),
	lineDef("", "İştiraklerin Dağıtılmamış Karları ile İlgili Düzeltmeler", "Adjustments related to undistributed profits of associates", StatementCashFlow, "associate_undistributed_profit_adjustments"),
	lineDef("", "Vergi Gideri/Geliri ile İlgili Düzeltmeler", "Tax expense/income adjustments", StatementCashFlow, "tax_expense_income_adjustments"),
	lineDef("", "Duran Varlıkların Elden Çıkarılmasından Kaynaklanan Kayıp/Kazançlar ile İlgili Düzeltmeler", "Adjustments for gains/losses on disposal of non-current assets", StatementCashFlow, "non_current_asset_disposal_gain_loss_adjustments"),
	lineDef("", "Yatırım ya da Finansman Faaliyetlerinden Kaynaklanan Nakit Akışlarına Neden Olan Diğer Kalemlere İlişkin Düzeltmeler", "Adjustments for other items causing investing or financing cash flows", StatementCashFlow, "other_investing_financing_cash_flow_adjustments"),
	lineDef("", "Kar/Zarar Mutabakatı ile İlgili Diğer Düzeltmeler", "Other profit/loss reconciliation adjustments", StatementCashFlow, "other_profit_loss_reconciliation_adjustments"),
	lineDef("", "İşletme Sermayesinde Gerçekleşen Değişimler", "Working capital changes", StatementCashFlow, "working_capital_changes"),
	lineDef("", "Stoklardaki Artış/Azalışla İlgili Düzeltmeler", "Adjustments for increase/decrease in inventories", StatementCashFlow, "inventory_change_adjustments"),
	lineDef("", "Ticari Alacaklardaki Artış/Azalışla İlgili Düzeltmeler", "Adjustments for increase/decrease in trade receivables", StatementCashFlow, "trade_receivable_change_adjustments"),
	lineDef("", "Finans Sektörü Faaliyetlerinden Alacaklarda Artış/Azalış", "Increase/decrease in financial sector receivables", StatementCashFlow, "financial_sector_receivable_change_adjustments"),
	lineDef("", "Faaliyetlerle İlgili Diğer Alacaklardaki Artış/Azalışla İlgili Düzeltmeler", "Adjustments for increase/decrease in other operating receivables", StatementCashFlow, "other_operating_receivable_change_adjustments"),
	lineDef("", "Ticari Borçlardaki Artış/Azalışla İlgili Düzeltmeler", "Adjustments for increase/decrease in trade payables", StatementCashFlow, "trade_payable_change_adjustments"),
	lineDef("", "Finans Sektörü Faaliyetlerinden Borçlardaki Artış/Azalış", "Increase/decrease in financial sector payables", StatementCashFlow, "financial_sector_payable_change_adjustments"),
	lineDef("", "Faaliyetlerle İlgili Diğer Borçlardaki Artış/Azalışla İlgili Düzeltmeler", "Adjustments for increase/decrease in other operating payables", StatementCashFlow, "other_operating_payable_change_adjustments"),
	lineDef("", "İşletme Sermayesinde Gerçekleşen Diğer Artış/Azalışla İlgili Düzeltmeler", "Adjustments for other working capital increases/decreases", StatementCashFlow, "other_working_capital_change_adjustments"),
	lineDef("", "Şirket Tarafından Verilen TRİ’ler", "Guarantees, pledges and mortgages given by the company", StatementNote, "company_given_gpm"),
	lineDef("", "Kendi Tüzel Kişiliği Adına Vermiş Olduğu TRİ’lerin Toplam Tutarı", "Total GPM given on behalf of own legal entity", StatementNote, "gpm_own_legal_entity_total"),
	lineDef("", "Tam Konsolidasyon Kapsamına Dahil Edilen Ortaklıklar Lehine Vermiş Olduğu TRİ’lerin Toplam Tutarı", "Total GPM given on behalf of fully consolidated subsidiaries", StatementNote, "gpm_full_consolidation_subsidiaries_total"),
	lineDef("", "Olağan Ticari Faaliyetlerinin Yürütülmesi Amacıyla Diğer 3. Kişilerin Borcunu Temin Amacıyla Vermiş Olduğu TRİ’lerin Toplam Tutarı", "Total GPM given for third-party debts in ordinary operations", StatementNote, "gpm_third_party_ordinary_operations_total"),
	lineDef("", "Diğer Verilen TRİ’lerin Toplam Tutarı", "Other GPM total", StatementNote, "gpm_other_total"),
	lineDef("", "Ana Ortak Lehine Vermiş Olduğu TRİ’lerin Toplam Tutarı", "GPM total given on behalf of parent", StatementNote, "gpm_parent_total"),
	lineDef("", "B ve C maddeleri Kapsamına Girmeyen Diğer Grup Şirketleri Lehine Vermiş Olduğu TRİ’lerin Toplam Tutarı", "GPM total given on behalf of other group companies outside B and C", StatementNote, "gpm_other_group_companies_total"),
	lineDef("", "C Maddesi Kapsamına Girmeyen 3. Kişiler Lehine Vermiş Olduğu TRİ’lerin Toplam Tutarı", "GPM total given on behalf of third parties outside C", StatementNote, "gpm_other_third_parties_total"),
	lineDef("", "Raporlama tarihi itibariyle maruz kalınan azami kredi riski", "Maximum credit risk exposure at reporting date", StatementNote, "maximum_credit_risk_exposure"),
	lineDef("", "Azami riskin teminat, vs ile güvence altına alınmış kısmı", "Portion of maximum risk secured by collateral", StatementNote, "maximum_risk_secured_by_collateral"),
	lineDef("", "Vadesi geçmemiş ya da değer düşüklüğüne uğramamış finansal varlıkların net defter değeri", "Net carrying amount of financial assets neither past due nor impaired", StatementNote, "not_past_due_not_impaired_financial_assets"),
	lineDef("", "Vadesi geçmiş ancak değer düşüklüğüne uğramamış varlıkların net defter değeri", "Net carrying amount of past due but not impaired assets", StatementNote, "past_due_not_impaired_assets"),
	lineDef("", "Değer düşüklüğüne uğrayan varlıkların net defter değerleri", "Net carrying amount of impaired assets", StatementNote, "impaired_assets_net_carrying_amount"),
	lineDef("", "Vadesi geçmiş brüt defter değeri", "Past due gross carrying amount", StatementNote, "past_due_gross_carrying_amount"),
	lineDef("", "Değer düşüklüğü", "Impairment", StatementNote, "impairment"),
	lineDef("", "Net değerin teminat, vs ile güvence altına alınmış kısmı", "Portion of net amount secured by collateral", StatementNote, "net_amount_secured_by_collateral"),
	lineDef("", "Vadesi geçmemiş brüt defter değeri", "Not past due gross carrying amount", StatementNote, "not_past_due_gross_carrying_amount"),
	lineDef("", "Finansal durum tablosu dışı kredi riski içeren unsurlar", "Off-balance-sheet items containing credit risk", StatementNote, "off_balance_sheet_credit_risk_items"),
	lineDef("", "Vadesi üzerinden 1-30 gün geçmiş", "1-30 days past due", StatementNote, "past_due_1_30_days"),
	lineDef("", "Vadesi üzerinden 1-3 ay geçmiş", "1-3 months past due", StatementNote, "past_due_1_3_months"),
	lineDef("", "Vadesi üzerinden 3-12 ay geçmiş", "3-12 months past due", StatementNote, "past_due_3_12_months"),
	lineDef("", "Vadesi üzerinden 1-5 yıl geçmiş", "1-5 years past due", StatementNote, "past_due_1_5_years"),
	lineDef("", "Vadesini 5 yıldan fazla geçmiş", "More than 5 years past due", StatementNote, "past_due_more_than_5_years"),
	lineDef("", "Sözleşme uyarınca vadeler", "Contractual maturities", StatementNote, "contractual_maturities"),
	lineDef("", "Beklenen Vadeler", "Expected maturities", StatementNote, "expected_maturities"),
	lineDef("", "Defter Değeri", "Carrying amount", StatementNote, "carrying_amount"),
	lineDef("", "Sözleşme uyarınca nakit çıkışlar toplamı", "Total contractual cash outflows", StatementNote, "total_contractual_cash_outflows"),
	lineDef("", "Beklenen nakit çıkışlar toplamı", "Total expected cash outflows", StatementNote, "total_expected_cash_outflows"),
	lineDef("", "Türev Olmayan Finansal Yükümlülükler", "Non-derivative financial liabilities", StatementNote, "non_derivative_financial_liabilities"),
	lineDef("", "Banka kredileri", "Bank loans", StatementNote, "bank_loans"),
	lineDef("", "Borçlanma senedi ihraçları", "Issued debt securities", StatementNote, "issued_debt_securities"),
	lineDef("", "Finansal kiralama yükümlülükleri", "Finance lease liabilities", StatementNote, "finance_lease_liabilities"),
	lineDef("", "Türev Finansal Yükümlülükler Net", "Derivative financial liabilities net", StatementNote, "derivative_financial_liabilities_net"),
	lineDef("", "Türev Nakit Girişleri", "Derivative cash inflows", StatementNote, "derivative_cash_inflows"),
	lineDef("", "Türev Nakit Çıkışları", "Derivative cash outflows", StatementNote, "derivative_cash_outflows"),
	lineDef("", "Likidite Riski Açıklamaları", "Liquidity risk disclosures", StatementNote, "liquidity_risk_disclosures"),
	lineDef("", "Sözleşme uyarınca nakit çıkışlar", "Contractual cash outflows", StatementNote, "contractual_cash_outflows"),
	lineDef("", "Sözleşme uyarınca/Beklenen nakit çıkışlar toplamı", "Total contractual/expected cash outflows", StatementNote, "contractual_expected_cash_outflows_total"),
	lineDef("", "3 aydan kısa", "Less than 3 months", StatementNote, "maturity_less_than_3_months"),
	lineDef("", "3-12 ay arası", "3-12 months", StatementNote, "maturity_3_12_months"),
	lineDef("", "1-5 yıl arası", "1-5 years", StatementNote, "maturity_1_5_years"),
	lineDef("", "5 yıldan uzun", "More than 5 years", StatementNote, "maturity_more_than_5_years"),
	lineDef("", "1-2 yıl arası", "1-2 years", StatementNote, "maturity_1_2_years"),
	lineDef("", "2 yıldan uzun", "More than 2 years", StatementNote, "maturity_more_than_2_years"),
	lineDef("", "Beklenen veya sözleşme uyarınca vadeler", "Expected or contractual maturities", StatementNote, "expected_or_contractual_maturities", "Beklenen (veya sözleşme uyarınca) vadeler"),
	lineDef("", "Finansal kiralama yükümlülükleri", "Finance lease liabilities", StatementNote, "finance_lease_liabilities"),
	lineDef("", "Likidite Riski Diğer Borçlar", "Liquidity risk other payables", StatementNote, "liquidity_other_payables", "Diğer borçlar"),
	lineDef("", "Finansal Araçlardan Kaynaklanan Risklerin Niteliği ve Düzeyi", "Nature and extent of risks arising from financial instruments", StatementNote, "financial_instrument_risk_nature_extent"),
	lineDef("", "Kredi Riski Açıklamaları", "Credit risk disclosures", StatementNote, "credit_risk_disclosures"),
	lineDef("", "Likidite Riskine İlişkin Açıklamalar", "Liquidity risk disclosures", StatementNote, "liquidity_risk_related_disclosures"),
	lineDef("", "Piyasa Riski Açıklamaları", "Market risk disclosures", StatementNote, "market_risk_disclosures"),
	lineDef("", "Alacaklar", "Receivables", StatementNote, "receivables"),
	lineDef("", "Ticari Alacaklar", "Trade receivables", StatementNote, "note_trade_receivables"),
	lineDef("", "Diğer Alacaklar", "Other receivables", StatementNote, "note_other_receivables"),
	lineDef("", "Bankalardaki Mevduat", "Deposits at banks", StatementNote, "bank_deposits", "Mevduat"),
	lineDef("", "Finansal Araç Türleri İtibariyle Maruz Kalınan Kredi Riskleri", "Credit risks exposed by financial instrument types", StatementNote, "credit_risk_by_financial_instrument_type"),
	lineDef("", "TFRS 7 referansı", "TFRS 7 reference", StatementNote, "tfrs7_reference", "TFRS 7 referasnsı"),
	lineDef("", "İlişkili Taraf", "Related party", StatementNote, "related_party_column"),
	lineDef("", "Diğer Taraf", "Other party", StatementNote, "other_party_column"),
	lineDef("", "Finansal Araçlar", "Financial instruments", StatementNote, "financial_instruments"),
	lineDef("", "Finansal Varlıklar", "Financial assets", StatementNote, "financial_assets"),
	lineDef("", "Döviz Pozisyonu Tablosu", "Foreign currency position table", StatementNote, "fx_position_table"),
	lineDef("", "Türk Lirası Karşılığı", "Turkish lira equivalent", StatementNote, "try_equivalent_column", "Tük Lirası Karşılığı", "Fonksiyonel para birimi"),
	lineDef("", "ABD Doları", "US Dollar", StatementNote, "usd_column", "ABD Doları", "Doları"),
	lineDef("", "Avro", "Euro", StatementNote, "eur_column"),
	lineDef("", "Yen", "Yen", StatementNote, "jpy_column"),
	lineDef("", "GBP", "GBP", StatementNote, "gbp_column"),
	lineDef("", "Diğer Döviz", "Other currency", StatementNote, "other_currency_column"),
	lineDef("", "Döviz Pozisyonu Diğer", "Foreign currency position other", StatementNote, "fx_position_other", "Diğer"),
	lineDef("", "Döviz Pozisyonu Dönen Varlıklar", "FX position current assets", StatementNote, "fx_current_assets", "Dönen Varlıklar (1+2+3)"),
	lineDef("", "Döviz Pozisyonu Duran Varlıklar", "FX position non-current assets", StatementNote, "fx_non_current_assets", "Duran Varlıklar (5+6+7)"),
	lineDef("", "Döviz Pozisyonu Toplam Varlıklar", "FX position total assets", StatementNote, "fx_total_assets", "Toplam Varlıklar (4+8)"),
	lineDef("", "Döviz Pozisyonu Kısa Vadeli Yükümlülükler", "FX position current liabilities", StatementNote, "fx_current_liabilities", "Kısa Vadeli Yükümlükler (10+11+12)", "Kısa Vadeli Yükümlülükler (10+11+12)"),
	lineDef("", "Döviz Pozisyonu Uzun Vadeli Yükümlülükler", "FX position non-current liabilities", StatementNote, "fx_non_current_liabilities", "Uzun Vadeli Yükümlülükler (14+15+16)"),
	lineDef("", "Döviz Pozisyonu Toplam Yükümlülükler", "FX position total liabilities", StatementNote, "fx_total_liabilities", "Toplam Yükümlülükler (13+17)"),
	lineDef("", "Döviz Pozisyonu Ticari Borçlar", "FX position trade payables", StatementNote, "fx_trade_payables"),
	lineDef("", "Döviz Pozisyonu Finansal Yükümlülükler", "FX position financial liabilities", StatementNote, "fx_financial_liabilities"),
	lineDef("", "Parasal Finansal Varlıklar", "Monetary financial assets", StatementNote, "monetary_financial_assets"),
	lineDef("", "Parasal Olmayan Finansal Varlıklar", "Non-monetary financial assets", StatementNote, "non_monetary_financial_assets"),
	lineDef("", "Toplam Varlıklar", "Total assets", StatementNote, "note_total_assets"),
	lineDef("", "Finansal Yükümlülükler", "Financial liabilities", StatementNote, "financial_liabilities"),
	lineDef("", "Parasal Olan Diğer Yükümlülükler", "Other monetary liabilities", StatementNote, "other_monetary_liabilities"),
	lineDef("", "Parasal Olmayan Diğer Yükümlülükler", "Other non-monetary liabilities", StatementNote, "other_non_monetary_liabilities"),
	lineDef("", "Toplam Yükümlülükler", "Total liabilities", StatementNote, "total_liabilities"),
	lineDef("", "Finansal durum tablosu Dışı Döviz Cinsinden Türev Araçların Net Varlık / Yükümlülük Pozisyonu", "Net asset/liability position of off-balance-sheet FX derivatives", StatementNote, "off_balance_sheet_fx_derivatives_net_position"),
	lineDef("", "Aktif Karakterli Finansal durum tablosu Dışı Döviz Cinsinden Türev Ürünlerin Tutarı", "Asset-character off-balance-sheet FX derivative amount", StatementNote, "asset_character_off_balance_sheet_fx_derivatives"),
	lineDef("", "Pasif Karakterli Finansal durum tablosu Dışı Döviz Cinsinden Türev Ürünlerin Tutarı", "Liability-character off-balance-sheet FX derivative amount", StatementNote, "liability_character_off_balance_sheet_fx_derivatives"),
	lineDef("", "Net Yabancı Para Varlık / Yükümlülük Pozisyonu", "Net foreign currency asset/liability position", StatementNote, "net_foreign_currency_asset_liability_position"),
	lineDef("", "Parasal Kalemler Net Yabancı Para Varlık / Yükümlülük Pozisyonu", "Net monetary foreign currency asset/liability position", StatementNote, "net_monetary_foreign_currency_asset_liability_position"),
	lineDef("", "Döviz Hedge'i İçin Kullanılan Finansal Araçların Toplam Gerçeğe Uygun Değeri", "Total fair value of financial instruments used for FX hedge", StatementNote, "fx_hedge_instruments_total_fair_value"),
	lineDef("", "Döviz Varlıkların Hedge Edilen Kısmının Tutarı", "Hedged portion of FX assets", StatementNote, "hedged_fx_assets_amount"),
	lineDef("", "Döviz Yükümlülüklerin Hedge Edilen Kısmının Tutarı", "Hedged portion of FX liabilities", StatementNote, "hedged_fx_liabilities_amount"),
	lineDef("", "ABD Doları net varlık/yükümlülüğü", "USD net asset/liability", StatementNote, "usd_net_asset_liability"),
	lineDef("", "ABD Doları riskinden korunan kısım", "USD hedged portion", StatementNote, "usd_hedged_portion"),
	lineDef("", "ABD Doları Net Etki", "USD net effect", StatementNote, "usd_net_effect"),
	lineDef("", "ABD Doları kurunun değişmesi halinde", "If USD exchange rate changes", StatementNote, "usd_exchange_rate_change_scenario", "ABD Doları kurunun % değişmesi halinde"),
	lineDef("", "Avro net varlık/yükümlülüğü", "EUR net asset/liability", StatementNote, "eur_net_asset_liability"),
	lineDef("", "Avro riskinden korunan kısım", "EUR hedged portion", StatementNote, "eur_hedged_portion"),
	lineDef("", "Avro Net Etki", "EUR net effect", StatementNote, "eur_net_effect"),
	lineDef("", "Avro kurunun değişmesi halinde", "If EUR exchange rate changes", StatementNote, "eur_exchange_rate_change_scenario", "Avro kurunun % değişmesi halinde"),
	lineDef("", "Diğer döviz net varlık/yükümlülüğü", "Other currency net asset/liability", StatementNote, "other_currency_net_asset_liability"),
	lineDef("", "Diğer döviz kuru riskinden korunan kısım", "Other currency hedged portion", StatementNote, "other_currency_hedged_portion"),
	lineDef("", "Diğer Döviz Varlıkları Net Etki", "Other currency net effect", StatementNote, "other_currency_net_effect"),
	lineDef("", "Diğer döviz kurlarının ortalama değişmesi halinde", "If other exchange rates change on average", StatementNote, "other_exchange_rates_average_change_scenario", "Diğer döviz kurlarının ortalama % değişmesi halinde"),
	lineDef("", "Döviz Kuru Duyarlılık Analizi Tablosu", "Foreign exchange sensitivity analysis table", StatementNote, "fx_sensitivity_analysis_table"),
	lineDef("", "Cari Dönem", "Current period", StatementNote, "current_period_column"),
	lineDef("", "Önceki Dönem", "Previous period", StatementNote, "previous_period_column"),
	lineDef("", "Kar/Zarar", "Profit or loss", StatementNote, "profit_loss_column"),
	lineDef("", "Özkaynaklar", "Equity", StatementNote, "equity_column"),
	lineDef("", "Yabancı paranın değer kazanması", "Foreign currency appreciation", StatementNote, "foreign_currency_appreciation_column"),
	lineDef("", "Yabancı paranın değer kaybetmesi", "Foreign currency depreciation", StatementNote, "foreign_currency_depreciation_column"),
	lineDef("", "TOPLAM (3+6+9)", "Total (3+6+9)", StatementNote, "fx_sensitivity_total"),
	lineDef("", "Sabit faizli finansal araçlar", "Fixed-rate financial instruments", StatementNote, "fixed_rate_financial_instruments"),
	lineDef("", "Faiz Pozisyonu Tablosu", "Interest rate position table", StatementNote, "interest_rate_position_table"),
	lineDef("", "Değişken faizli finansal araçlar", "Variable-rate financial instruments", StatementNote, "variable_rate_financial_instruments"),
	lineDef("", "Gerçeğe uygun değer farkı kar/zarara yansıtılan varlıklar", "Assets at fair value through profit or loss", StatementNote, "assets_fvtpl"),
	lineDef("", "Satılmaya hazır finansal varlıklar", "Available-for-sale financial assets", StatementNote, "available_for_sale_financial_assets"),
	lineDef("", "Diğer Risklere İlişkin Duyarlılık Analizi", "Sensitivity analysis for other risks", StatementNote, "other_risks_sensitivity_analysis"),
	lineDef("", "Finansal Araçlar Gerçeğe Uygun Değer Açıklamaları ve Finansal Riskten Korunma Muhasebesi Çerçevesindeki Açıklamalar", "Fair value and hedge accounting disclosures for financial instruments", StatementNote, "financial_instruments_fair_value_hedge_accounting_disclosures"),
	lineDef("", "Raporlama Döneminden Sonraki Olaylar", "Events after the reporting period", StatementNote, "events_after_reporting_period"),
	lineDef("", "Finansal tabloların önemli ölçüde etkileyen ya da finansal tabloların açık, yorumlanabilir ve anlaşılabilir olması açısından açıklanması gereken diğer hususlar", "Other matters materially affecting or needed for clear financial statements", StatementNote, "other_material_financial_statement_matters"),
	lineDef("", "Ek Dipnot: Portföy Sınırlamalarına Uyumun Kontrolü", "Additional note: portfolio limitation compliance check", StatementNote, "portfolio_limit_compliance_note"),
	lineDef("", "Portföy Sınırlamalarına Uyumun Kontrolü", "Portfolio limitation compliance check", StatementNote, "portfolio_limit_compliance_check"),
	lineDef("", "Konsolide Olmayan Bireysel Finansal Tablo Ana Hesap Kalemleri", "Unconsolidated standalone financial statement main account lines", StatementNote, "standalone_main_account_lines", "Konsolide Olmayan (Bireysel) Finansal Tablo Ana Hesap Kalemleri"),
	lineDef("", "Konsolide Olmayan Bireysel Diğer Finansal Bilgiler", "Unconsolidated standalone other financial information", StatementNote, "standalone_other_financial_information", "Konsolide Olmayan (Bireysel) Diğer Finansal Bilgiler"),
	lineDef("", "İlgili Düzenleme", "Relevant regulation", StatementNote, "relevant_regulation_column"),
	lineDef("", "Asgari/Azami Oran", "Minimum/maximum ratio", StatementNote, "minimum_maximum_ratio_column"),
	lineDef("", "Para ve Sermaye Piyasası Araçları", "Money and capital market instruments", StatementNote, "money_and_capital_market_instruments"),
	lineDef("", "Gayrimenkuller, Gayrimenkule Dayalı Projeler, Gayrimenkule Dayalı Haklar", "Real estate, real-estate projects and real-estate-based rights", StatementNote, "real_estate_projects_rights"),
	lineDef("", "İştirakler", "Associates", StatementNote, "associates"),
	lineDef("", "İlişkili Taraflardan Alacaklar Ticari Olmayan", "Non-trade receivables from related parties", StatementNote, "related_party_non_trade_receivables"),
	lineDef("", "Diğer Varlıklar", "Other assets", StatementNote, "note_other_assets"),
	lineDef("", "Diğer Kaynaklar", "Other liabilities/equity sources", StatementNote, "note_other_sources"),
	lineDef("", "Toplam Varlıklar Aktif Toplamı", "Total assets active total", StatementNote, "portfolio_total_assets_active_total", "Toplam Varlıklar (Aktif Toplamı)"),
	lineDef("", "Finansal Borçlar", "Financial debt", StatementNote, "note_financial_debt"),
	lineDef("", "Finansal Kiralama Borçları", "Finance lease debts", StatementNote, "finance_lease_debts"),
	lineDef("", "İlişkili Taraflara Borçlar Ticari Olmayan", "Non-trade payables to related parties", StatementNote, "related_party_non_trade_payables", "İlişkili Taraflara Borçlar (Ticari Olmayan)"),
	lineDef("", "Para ve Sermaye Piyasası Araçlarının 3 yıllık Gayrimenkul Ödemeleri İçin Tutulan Kısmı", "Money/capital market instruments held for three-year real estate payments", StatementNote, "money_capital_market_instruments_for_three_year_real_estate_payments"),
	lineDef("", "Vadeli/Vadesiz TL/Döviz", "Time/demand TL/FX deposits", StatementNote, "time_demand_tl_fx"),
	lineDef("", "Yabancı Sermaye Piyasası Araçları", "Foreign capital market instruments", StatementNote, "foreign_capital_market_instruments"),
	lineDef("", "Yabancı Gayrimenkuller, Gayrimenkule Dayalı Projeler, Gayrimenkule Dayalı Haklar", "Foreign real estate, real-estate projects and real-estate-based rights", StatementNote, "foreign_real_estate_projects_rights"),
	lineDef("", "Atıl Tutulan Arsa/Araziler", "Idle land/plots", StatementNote, "idle_land_plots"),
	lineDef("", "Yabancı İştirakler", "Foreign associates", StatementNote, "foreign_associates"),
	lineDef("", "İşletmeci Şirkete İştirak", "Participation in operator company", StatementNote, "operator_company_participation"),
	lineDef("", "Gayrinakdi Krediler", "Non-cash loans", StatementNote, "non_cash_loans"),
	lineDef("", "Üzerinde proje geliştirilecek mülkiyeti ortaklığa ait olmayan ipotekli arsaların ipotek bedelleri", "Mortgage amounts of mortgaged land not owned by partnership for project development", StatementNote, "mortgage_amounts_of_non_owned_project_land"),
	lineDef("", "Yabancı Gayrimenkuller, Gayrimenkule Dayalı Projeler, Gayrimenkule Dayalı Haklar, İştirakler, Sermaye Piyasası Araçları", "Foreign real estate, real-estate projects, rights, associates and capital market instruments", StatementNote, "foreign_real_estate_associates_capital_market_instruments"),
	lineDef("", "Borçlanma Sınırı", "Debt limit", StatementNote, "debt_limit"),
	lineDef("", "Portföy Sınırlamaları", "Portfolio limitations", StatementNote, "portfolio_limitations"),
}

func lineDef(code, tr, en, statementType, normalized string, aliases ...string) LineDefinition {
	return LineDefinition{
		Code:          code,
		DescTR:        tr,
		DescEN:        en,
		Normalized:    normalized,
		StatementType: statementType,
		Aliases:       aliases,
	}
}

func CanonicalLineDefinitions() []LineDefinition {
	out := make([]LineDefinition, len(canonicalFinancialLineCatalog))
	copy(out, canonicalFinancialLineCatalog)
	return out
}

func CanonicalFinancialLineTerms() []string {
	seen := map[string]struct{}{}
	out := []string{}
	add := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		slug := util.SlugTR(value)
		if slug == "" {
			return
		}
		if _, ok := seen[slug]; ok {
			return
		}
		seen[slug] = struct{}{}
		out = append(out, value)
	}
	for _, def := range canonicalFinancialLineCatalog {
		add(def.DescTR)
		add(def.DescEN)
		add(def.Normalized)
		for _, alias := range def.Aliases {
			add(alias)
		}
	}
	for _, key := range canonicalFinancialLineSearchKeys() {
		add(key.slug)
	}
	return out
}

func CanonicalLineDefinitionForText(value string) (LineDefinition, bool) {
	line, ok := canonicalLineForTextCatalog(value)
	return LineDefinition(line), ok
}

func CanonicalLineDefinitionForTextInContext(value, context string) (LineDefinition, bool) {
	if def, ok := contextualFinancialLineDefinition(value, context); ok {
		return def, true
	}
	return CanonicalLineDefinitionForText(value)
}

func StatementTypeForNormalizedLine(normalized string) string {
	if def, ok := canonicalLineForTextCatalog(normalized); ok {
		return def.StatementType
	}
	return ""
}

func canonicalLineForTextCatalog(value string) (canonicalLine, bool) {
	slug := util.SlugTR(strings.TrimSpace(value))
	if slug == "" {
		return canonicalLine{}, false
	}
	if strings.Contains(slug, "finanssektorufaaliyetlerindenbrut") &&
		strings.Contains(slug, "finansmangiderioncesifaaliyet") {
		for _, def := range canonicalFinancialLineCatalog {
			if def.Code == "3CAFA" {
				return def, true
			}
		}
	}
	for _, key := range canonicalFinancialLineSearchKeys() {
		if slug == key.slug {
			return key.def, true
		}
	}
	for _, key := range canonicalFinancialLineSearchKeys() {
		if canonicalSearchKeyMayMatchInside(key.slug, slug) && strings.Contains(slug, key.slug) {
			return key.def, true
		}
	}
	return canonicalLine{}, false
}

func canonicalFinancialLineSearchKeys() []canonicalLineSearchKey {
	canonicalFinancialLineSearchKeysOnce.Do(func() {
		canonicalFinancialLineSearchKeysCache = buildCanonicalFinancialLineSearchKeys()
	})
	return canonicalFinancialLineSearchKeysCache
}

func buildCanonicalFinancialLineSearchKeys() []canonicalLineSearchKey {
	keys := []canonicalLineSearchKey{}
	seen := map[string]struct{}{}
	add := func(def LineDefinition, value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		slug := util.SlugTR(value)
		if slug == "" {
			return
		}
		mapKey := def.Normalized + "|" + slug
		if _, ok := seen[mapKey]; ok {
			return
		}
		seen[mapKey] = struct{}{}
		keys = append(keys, canonicalLineSearchKey{def: def, slug: slug})
	}
	for _, def := range canonicalFinancialLineCatalog {
		add(def, def.DescTR)
		add(def, def.DescEN)
		add(def, def.Normalized)
		for _, alias := range def.Aliases {
			add(def, alias)
		}
	}
	sort.SliceStable(keys, func(i, j int) bool {
		return len(keys[i].slug) > len(keys[j].slug)
	})
	return keys
}

func canonicalSearchKeyMayMatchInside(keySlug, lineSlug string) bool {
	if keySlug == "" || lineSlug == "" || keySlug == lineSlug {
		return true
	}
	if len(keySlug) < 12 {
		return false
	}
	for _, blocked := range []string{
		"stoklar", "hasilat", "ticari borclar", "ticari alacaklar", "finansal borclar",
		"finansman giderleri", "finansman gelirleri", "donem karizarari",
	} {
		if keySlug == blocked {
			return false
		}
	}
	return true
}

func contextualFinancialLineDefinition(value, context string) (LineDefinition, bool) {
	valueSlug := util.SlugTR(value)
	contextSlug := util.SlugTR(context)
	if valueSlug == "" || contextSlug == "" {
		return LineDefinition{}, false
	}
	if strings.Contains(contextSlug, "creditrisk") {
		switch valueSlug {
		case "alacaklar":
			return supplementalContextLine("Kredi Riski Alacaklar", "Credit risk receivables", StatementNote, "credit_risk_receivables"), true
		case "ticarialacaklar":
			return supplementalContextLine("Kredi Riski Ticari Alacaklar", "Credit risk trade receivables", StatementNote, "credit_risk_trade_receivables"), true
		case "digeralacaklar":
			return supplementalContextLine("Kredi Riski Diğer Alacaklar", "Credit risk other receivables", StatementNote, "credit_risk_other_receivables"), true
		case "iliskilitaraf":
			return supplementalContextLine("Kredi Riski İlişkili Taraf", "Credit risk related party column", StatementNote, "credit_risk_related_party_column"), true
		case "digertaraf":
			return supplementalContextLine("Kredi Riski Diğer Taraf", "Credit risk other party column", StatementNote, "credit_risk_other_party_column"), true
		case "bankalardakimevduat", "mevduat":
			return supplementalContextLine("Kredi Riski Bankalardaki Mevduat", "Credit risk bank deposits", StatementNote, "credit_risk_bank_deposits"), true
		case "turevaraclar":
			return supplementalContextLine("Kredi Riski Türev Araçlar", "Credit risk derivative instruments", StatementNote, "credit_risk_derivative_instruments"), true
		case "diger":
			return supplementalContextLine("Kredi Riski Diğer", "Credit risk other column", StatementNote, "credit_risk_other_column"), true
		}
	}
	if strings.Contains(contextSlug, "liquidityrisk") || strings.Contains(contextSlug, "likiditeriski") {
		switch valueSlug {
		case "ticariborclar":
			return supplementalContextLine("Likidite Riski Ticari Borçlar", "Liquidity risk trade payables", StatementNote, "liquidity_trade_payables"), true
		case "digerborclar":
			return supplementalContextLine("Likidite Riski Diğer Borçlar", "Liquidity risk other payables", StatementNote, "liquidity_other_payables"), true
		case "bankakredileri":
			return supplementalContextLine("Likidite Riski Banka Kredileri", "Liquidity risk bank loans", StatementNote, "liquidity_bank_loans"), true
		case "borclanmasenedihraclari", "borclanmasenediihraclari":
			return supplementalContextLine("Likidite Riski Borçlanma Senedi İhraçları", "Liquidity risk issued debt securities", StatementNote, "liquidity_issued_debt_securities"), true
		case "finansalkiralamayukumlulukleri":
			return supplementalContextLine("Likidite Riski Finansal Kiralama Yükümlülükleri", "Liquidity risk finance lease liabilities", StatementNote, "liquidity_finance_lease_liabilities"), true
		}
	}
	if strings.Contains(contextSlug, "fxposition") || strings.Contains(contextSlug, "dovizpozisyon") {
		switch valueSlug {
		case "ticarialacaklar":
			return supplementalContextLine("Döviz Pozisyonu Ticari Alacaklar", "FX position trade receivables", StatementNote, "fx_trade_receivables"), true
		case "ticariborclar":
			return supplementalContextLine("Döviz Pozisyonu Ticari Borçlar", "FX position trade payables", StatementNote, "fx_trade_payables"), true
		case "finansalyukumlulukler":
			return supplementalContextLine("Döviz Pozisyonu Finansal Yükümlülükler", "FX position financial liabilities", StatementNote, "fx_financial_liabilities"), true
		case "diger":
			return supplementalContextLine("Döviz Pozisyonu Diğer", "FX position other", StatementNote, "fx_position_other"), true
		case "toplamvarliklar":
			return supplementalContextLine("Döviz Pozisyonu Toplam Varlıklar", "FX position total assets", StatementNote, "fx_total_assets"), true
		case "toplamyukumlulukler":
			return supplementalContextLine("Döviz Pozisyonu Toplam Yükümlülükler", "FX position total liabilities", StatementNote, "fx_total_liabilities"), true
		}
	}
	if strings.Contains(contextSlug, "fxsensitivity") || strings.Contains(contextSlug, "dovizkuruduyarlilik") {
		switch valueSlug {
		case "caridonem":
			return supplementalContextLine("Döviz Duyarlılık Cari Dönem", "FX sensitivity current period", StatementNote, "fx_sensitivity_current_period_column"), true
		case "oncekidonem":
			return supplementalContextLine("Döviz Duyarlılık Önceki Dönem", "FX sensitivity previous period", StatementNote, "fx_sensitivity_previous_period_column"), true
		case "karzarar":
			return supplementalContextLine("Döviz Duyarlılık Kar/Zarar", "FX sensitivity profit/loss", StatementNote, "fx_sensitivity_profit_loss_column"), true
		case "ozkaynaklar":
			return supplementalContextLine("Döviz Duyarlılık Özkaynaklar", "FX sensitivity equity", StatementNote, "fx_sensitivity_equity_column"), true
		}
	}
	if strings.Contains(contextSlug, "interestrateposition") || strings.Contains(contextSlug, "faizpozisyon") {
		switch valueSlug {
		case "finansalvarliklar":
			return supplementalContextLine("Faiz Pozisyonu Finansal Varlıklar", "Interest position financial assets", StatementNote, "interest_position_financial_assets"), true
		case "finansalyukumlulukler":
			return supplementalContextLine("Faiz Pozisyonu Finansal Yükümlülükler", "Interest position financial liabilities", StatementNote, "interest_position_financial_liabilities"), true
		}
	}
	if strings.Contains(contextSlug, "portfolio") || strings.Contains(contextSlug, "portfoy") {
		switch valueSlug {
		case "finansalborclar":
			return supplementalContextLine("Portföy Sınırlaması Finansal Borçlar", "Portfolio limits financial debt", StatementNote, "portfolio_financial_debt"), true
		case "digerfinansalyukumlulukler":
			return supplementalContextLine("Portföy Sınırlaması Diğer Finansal Yükümlülükler", "Portfolio limits other financial liabilities", StatementNote, "portfolio_other_financial_liabilities"), true
		case "finansalkiralamaborclari":
			return supplementalContextLine("Portföy Sınırlaması Finansal Kiralama Borçları", "Portfolio limits finance lease debts", StatementNote, "portfolio_finance_lease_debts"), true
		case "ozkaynaklar":
			return supplementalContextLine("Portföy Sınırlaması Özkaynaklar", "Portfolio limits equity", StatementNote, "portfolio_equity"), true
		case "toplamvarliklar", "toplamvarliklaraktiftoplami":
			return supplementalContextLine("Portföy Sınırlaması Toplam Varlıklar", "Portfolio limits total assets", StatementNote, "portfolio_total_assets"), true
		case "toplamkaynaklar":
			return supplementalContextLine("Portföy Sınırlaması Toplam Kaynaklar", "Portfolio limits total sources", StatementNote, "portfolio_total_sources"), true
		case "digervarliklar":
			return supplementalContextLine("Portföy Sınırlaması Diğer Varlıklar", "Portfolio limits other assets", StatementNote, "portfolio_other_assets"), true
		case "digerkaynaklar":
			return supplementalContextLine("Portföy Sınırlaması Diğer Kaynaklar", "Portfolio limits other sources", StatementNote, "portfolio_other_sources"), true
		}
	}
	if strings.Contains(contextSlug, "duranvarlik") || contextSlug == "noncurrentassets" || contextSlug == "longtermassets" {
		switch valueSlug {
		case "finansalyatirimlar":
			return findCanonicalLineByCode("1BC")
		case "ticarialacaklar":
			return findCanonicalLineByCode("1B")
		case "finanssektorufaaliyetlerindenalacaklar":
			return findCanonicalLineByCode("1BA")
		case "digeralacaklar":
			return findCanonicalLineByCode("1BB")
		case "musterisozlesmelerindendoganvarliklar":
			return findCanonicalLineByCode("1BBA")
		case "canlivarliklar":
			return findCanonicalLineByCode("1BE")
		case "stoklar":
			return findCanonicalLineByCode("1BFA")
		case "pesinodenmisgiderler":
			return supplementalContextLine("Peşin Ödenmiş Giderler", "Non-current prepaid expenses", StatementBalanceSheet, "non_current_prepaid_expenses"), true
		case "turevaraclar":
			return supplementalContextLine("Uzun Vadeli Türev Araçlar", "Non-current derivative instruments", StatementBalanceSheet, "non_current_derivative_instruments"), true
		}
	}
	if strings.Contains(contextSlug, "donenvarlik") || contextSlug == "currentassets" {
		switch valueSlug {
		case "finansalyatirimlar":
			return findCanonicalLineByCode("1AB")
		case "ticarialacaklar":
			return findCanonicalLineByCode("1AC")
		case "finanssektorufaaliyetlerindenalacaklar":
			return findCanonicalLineByCode("1AD")
		case "digeralacaklar":
			return findCanonicalLineByCode("1AE")
		case "musterisozlesmelerindendoganvarliklar":
			return findCanonicalLineByCode("1AEA")
		case "canlivarliklar":
			return findCanonicalLineByCode("1AG")
		case "stoklar":
			return findCanonicalLineByCode("1AF")
		case "pesinodenmisgiderler":
			return supplementalContextLine("Peşin Ödenmiş Giderler", "Current prepaid expenses", StatementBalanceSheet, "current_prepaid_expenses"), true
		case "turevaraclar":
			return supplementalContextLine("Kısa Vadeli Türev Araçlar", "Current derivative instruments", StatementBalanceSheet, "current_derivative_instruments"), true
		}
	}
	if strings.Contains(contextSlug, "uzunvadeliyukumluluk") || contextSlug == "noncurrentliabilities" || contextSlug == "longtermliabilities" {
		switch valueSlug {
		case "finansalborclar", "uzunvadeliborclanmalar":
			return findCanonicalLineByCode("2BA")
		case "digerfinansalyukumlulukler":
			return findCanonicalLineByCode("2BB")
		case "ticariborclar":
			return findCanonicalLineByCode("2BBA")
		case "digerborclar":
			return findCanonicalLineByCode("2BBB")
		case "musterisozlesmelerindendoganyukumlulukler":
			return findCanonicalLineByCode("2BBBA")
		case "finanssektorufaaliyetlerindenborclar":
			return findCanonicalLineByCode("2BC")
		case "devlettesvikveyardimlari":
			return findCanonicalLineByCode("2BD")
		case "ertelenmisgelirler":
			return findCanonicalLineByCode("2BDA")
		case "turevaraclar":
			return supplementalContextLine("Uzun Vadeli Türev Araçlar", "Non-current derivative liabilities", StatementBalanceSheet, "non_current_derivative_liabilities"), true
		}
	}
	if strings.Contains(contextSlug, "kisavadeliyukumluluk") || contextSlug == "currentliabilities" || contextSlug == "shorttermliabilities" {
		switch valueSlug {
		case "kisavadeliborclanmalar", "finansalborclar", "uzunvadeliborclanmalarinkisavadelikisimlari":
			return findCanonicalLineByCode("2AA")
		case "digerfinansalyukumlulukler":
			return findCanonicalLineByCode("2AAG")
		case "ticariborclar":
			return findCanonicalLineByCode("2AAGAA")
		case "digerborclar":
			return findCanonicalLineByCode("2AAGAB")
		case "musterisozlesmelerindendoganyukumlulukler":
			return findCanonicalLineByCode("2AAGAC")
		case "finanssektorufaaliyetlerindenborclar":
			return findCanonicalLineByCode("2AAGB")
		case "devlettesvikveyardimlari":
			return findCanonicalLineByCode("2AAGC")
		case "ertelenmisgelirler":
			return findCanonicalLineByCode("2AAGCA")
		case "turevaraclar":
			return supplementalContextLine("Kısa Vadeli Türev Araçlar", "Current derivative liabilities", StatementBalanceSheet, "current_derivative_liabilities"), true
		}
	}
	return LineDefinition{}, false
}

func findCanonicalLineByCode(code string) (LineDefinition, bool) {
	for _, def := range canonicalFinancialLineCatalog {
		if def.Code == code {
			return def, true
		}
	}
	return LineDefinition{}, false
}

func supplementalContextLine(tr, en, statementType, normalized string) LineDefinition {
	return LineDefinition{
		DescTR:        tr,
		DescEN:        en,
		StatementType: statementType,
		Normalized:    normalized,
	}
}
